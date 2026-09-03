package agent

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/common"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
)

const safeToolDigestMaxRunes = 900

// safeStructuralLabelRule maps concrete judgment/category strings found in
// audit-table tool output to short codes. Digests and fallback answers MUST
// emit only Code — never Needle — so recovery synthesis does not re-trigger
// provider content policy with long sensitive category phrases.
type safeStructuralLabelRule struct {
	Needle string
	Code   string
}

var safeStructuralLabelRules = []safeStructuralLabelRule{
	{Needle: "煽动颠覆国家政权、推翻社会主义制度", Code: "cat_subversion"},
	{Needle: "违反社会主义核心价值观", Code: "cat_values"},
	{Needle: "政治敏感话题", Code: "cat_political"},
	{Needle: "政治敏感", Code: "cat_political"},
	{Needle: "不安全", Code: "unsafe"},
	{Needle: "不通过", Code: "fail"},
	{Needle: "通过", Code: "pass"},
	{Needle: "安全", Code: "safe"},
	{Needle: "unsafe", Code: "unsafe"},
	{Needle: "safe", Code: "safe"},
	{Needle: "pass", Code: "pass"},
	{Needle: "fail", Code: "fail"},
}

var (
	knowledgeTitleAttrRe = regexp.MustCompile(`(?i)\btitle="([^"]+)"`)
	knowledgeNameTagRe   = regexp.MustCompile(`(?is)<name>\s*([^<]+?)\s*</name>`)
	contentTagRe         = regexp.MustCompile(`(?is)<content>(.*?)</content>`)
	xmlTagRe             = regexp.MustCompile(`(?s)<[^>]+>`)
	headerHintRe         = regexp.MustCompile(`(?i)(关键词|分类|判定|模型|标签|结论|status|label|category|verdict|model)`)
)

// recoverFromContentPolicy answers in the reply bubble after a provider safety
// refusal. It first retries LLM synthesis on redacted digests, then falls back
// to a deterministic title list — never EventError.
func (e *AgentEngine) recoverFromContentPolicy(
	ctx context.Context,
	query string,
	state *types.AgentState,
	sessionID string,
) {
	answerID := generateEventID("answer")
	// Drop any partial finalize answer already streamed in this turn.
	e.eventBus.Emit(ctx, event.Event{
		ID:        answerID,
		Type:      event.EventAgentFinalAnswer,
		SessionID: sessionID,
		Data: event.AgentFinalAnswerData{
			Content:         "",
			Done:            false,
			ReplacePrevious: true,
		},
	})
	safeMessages := e.buildSafeContentPolicyMessages(ctx, sessionID, query, state)
	fullAnswer, err := e.streamFinalAnswerMessages(ctx, sessionID, answerID, safeMessages)
	if err == nil && strings.TrimSpace(fullAnswer) != "" {
		state.FinalAnswer = fullAnswer
		logger.Infof(ctx, "[Agent][FinalAnswer] Safe content-policy recovery succeeded: %d chars", len(fullAnswer))
		common.PipelineInfo(ctx, "Agent", "final_answer_content_policy_recovered", map[string]interface{}{
			"session_id": sessionID,
			"answer_len": len(fullAnswer),
		})
		return
	}
	if err != nil {
		logger.Warnf(ctx, "[Agent][FinalAnswer] Safe recovery LLM also failed: %v", err)
	}
	e.emitContentPolicyFallback(ctx, query, state, sessionID)
}

func (e *AgentEngine) buildSafeContentPolicyMessages(
	ctx context.Context,
	sessionID, query string,
	state *types.AgentState,
) []chat.Message {
	preferZH := common.PrefersChinese(query)
	system := `You are a knowledge assistant. Answer the user's question using ONLY the redacted retrieval digests below.
Rules:
1. Write a normal knowledge answer in the reply body. Never mention content filters, API errors, risk control, blocked requests, or ask the user to rephrase.
2. Describe each document's purpose and overall conclusions at an abstract level.
3. If digests include label_counts / columns_guess for an audit table, you MAY cite those category-level stats (short codes like unsafe/cat_political) and multi-model agreement — WITHOUT listing concrete keywords.
4. Prefer one cohesive paragraph (or a short bullet list of documents) over dumping tables.
5. Match the user's language. Do not invent facts that are not supported by the digests.`
	if preferZH {
		system = `你是知识库问答助手。请仅根据下方【已脱敏】的检索摘要回答用户问题。
规则：
1. 用正常知识问答口吻直接写正文，禁止提及内容安全策略、风控、API 报错、拦截，也禁止让用户换一种问法。
2. 概括文档用途与整体结论；若摘要含 label_counts / columns_guess，可据此做类别级总结与多模型一致性说明，但不要复述具体词条。
3. 优先写成连贯段落（或按文档分点简述），不要贴表格。
4. 使用与用户问题相同的语言。摘要未支持的事实不要编造。`
	}

	messages := []chat.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: e.RenderUserTurnContent(sessionID, query)},
	}
	for _, step := range state.RoundSteps {
		for _, toolCall := range step.ToolCalls {
			if toolCall.Result == nil {
				continue
			}
			digest := buildSafeToolDigest(toolCall.Name, toolCall.Result)
			if digest == "" {
				continue
			}
			messages = append(messages, chat.Message{
				Role:    "user",
				Content: fmt.Sprintf("Retrieval digest from %s:\n%s", toolCall.Name, digest),
			})
		}
	}

	ask := fmt.Sprintf(`User question: %s

Write the final answer now as normal knowledge-base reply text.`, query)
	if preferZH {
		ask = fmt.Sprintf(`用户问题：%s

请直接写出最终回答正文（正常知识库答复，不要提风控/报错）。`, query)
	}
	return append(messages, chat.Message{Role: "user", Content: ask})
}

func buildSafeToolDigest(toolName string, result *types.ToolResult) string {
	if result == nil {
		return ""
	}
	var b strings.Builder
	title := ""
	if result.Data != nil {
		if v, ok := result.Data["knowledge_title"].(string); ok {
			title = strings.TrimSpace(v)
		}
		if title == "" {
			if v, ok := result.Data["title"].(string); ok {
				title = strings.TrimSpace(v)
			}
		}
		if v, ok := result.Data["total_chunks"]; ok {
			fmt.Fprintf(&b, "total_chunks=%v\n", v)
		}
		if v, ok := result.Data["fetched_chunks"]; ok {
			fmt.Fprintf(&b, "fetched_chunks=%v\n", v)
		}
	}
	if title == "" {
		if m := knowledgeTitleAttrRe.FindStringSubmatch(result.Output); len(m) > 1 {
			title = strings.TrimSpace(m[1])
		}
	}
	if title != "" {
		fmt.Fprintf(&b, "title=%s\n", title)
	}
	fmt.Fprintf(&b, "tool=%s\n", toolName)

	if signals := extractStructuralSignals(result.Output); signals != "" {
		b.WriteString(signals)
		b.WriteString("\n")
	}

	abstract := redactToolOutputForSafeDigest(result.Output)
	if abstract != "" {
		b.WriteString("abstract:\n")
		b.WriteString(abstract)
		b.WriteString("\n")
	}
	out := strings.TrimSpace(b.String())
	return agenttools.TruncateToolOutput(out, safeToolDigestMaxRunes)
}

// extractStructuralSignals keeps column guesses and safe label counts from tool
// bodies without retaining concrete keyword rows.
func extractStructuralSignals(output string) string {
	if strings.TrimSpace(output) == "" {
		return ""
	}
	bodies := contentTagRe.FindAllStringSubmatch(output, -1)
	var combined strings.Builder
	for _, m := range bodies {
		if len(m) > 1 {
			combined.WriteString(m[1])
			combined.WriteByte('\n')
		}
	}
	text := combined.String()
	if text == "" {
		text = output
	}

	var parts []string
	if header := guessHeaderLine(text); header != "" {
		parts = append(parts, "columns_guess="+header)
	}
	if counts := countSafeLabels(text); counts != "" {
		parts = append(parts, "label_counts="+counts)
	}
	return strings.Join(parts, "\n")
}

func guessHeaderLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !headerHintRe.MatchString(line) {
			continue
		}
		// Prefer delimiter-looking header rows.
		if strings.Count(line, ",") >= 2 || strings.Count(line, "\t") >= 2 || strings.Count(line, "|") >= 2 {
			runes := []rune(line)
			if len(runes) > 120 {
				line = string(runes[:120]) + "…"
			}
			return line
		}
	}
	return ""
}

func countSafeLabels(text string) string {
	type kv struct {
		k string
		n int
	}
	// Longer needles first so "不安全" is not double-counted as "安全".
	rules := append([]safeStructuralLabelRule(nil), safeStructuralLabelRules...)
	sort.Slice(rules, func(i, j int) bool {
		return len([]rune(rules[i].Needle)) > len([]rune(rules[j].Needle))
	})
	remaining := text
	counts := make(map[string]int)
	for _, rule := range rules {
		n := strings.Count(remaining, rule.Needle)
		if n > 0 {
			counts[rule.Code] += n
			remaining = strings.ReplaceAll(remaining, rule.Needle, "")
		}
	}
	if len(counts) == 0 {
		return ""
	}
	items := make([]kv, 0, len(counts))
	for code, n := range counts {
		items = append(items, kv{code, n})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].n == items[j].n {
			return items[i].k < items[j].k
		}
		return items[i].n > items[j].n
	})
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, fmt.Sprintf("%s=%d", it.k, it.n))
	}
	return strings.Join(parts, ", ")
}

func redactToolOutputForSafeDigest(output string) string {
	if strings.TrimSpace(output) == "" {
		return ""
	}
	redacted := contentTagRe.ReplaceAllStringFunc(output, func(match string) string {
		inner := contentTagRe.FindStringSubmatch(match)
		body := ""
		if len(inner) > 1 {
			body = strings.TrimSpace(inner[1])
		}
		if body == "" {
			return "<content>(empty)</content>"
		}
		runes := []rune(body)
		return fmt.Sprintf(
			"<content>[body redacted for safe summary; original_len=%d; see label_counts/columns_guess above]</content>",
			len(runes),
		)
	})
	plain := strings.TrimSpace(xmlTagRe.ReplaceAllString(redacted, " "))
	plain = strings.Join(strings.Fields(plain), " ")
	if plain == "" {
		return strings.TrimSpace(redacted)
	}
	return agenttools.TruncateToolOutput(plain, 500)
}

// emitContentPolicyFallback publishes a deterministic in-bubble answer when even
// safe LLM recovery is unavailable. Titles only — no invented document semantics.
func (e *AgentEngine) emitContentPolicyFallback(
	ctx context.Context,
	query string,
	state *types.AgentState,
	sessionID string,
) {
	answer := buildContentPolicyFallbackAnswer(query, state)
	answerID := generateEventID("answer")
	e.eventBus.Emit(ctx, event.Event{
		ID:        answerID,
		Type:      event.EventAgentFinalAnswer,
		SessionID: sessionID,
		Data: event.AgentFinalAnswerData{
			Content:         answer,
			Done:            false,
			ReplacePrevious: true,
		},
	})
	e.eventBus.Emit(ctx, event.Event{
		ID:        answerID,
		Type:      event.EventAgentFinalAnswer,
		SessionID: sessionID,
		Data: event.AgentFinalAnswerData{
			Content: "",
			Done:    true,
		},
	})
	state.FinalAnswer = answer
	logger.Infof(ctx, "[Agent][FinalAnswer] Deterministic content-policy answer emitted: %d chars", len(answer))
}

func buildContentPolicyFallbackAnswer(query string, state *types.AgentState) string {
	titles := collectToolResultTitles(state)
	preferZH := common.PrefersChinese(query)

	// Prefer structural signals collected across tool outputs when available.
	var signalParts []string
	for _, step := range state.RoundSteps {
		for _, tc := range step.ToolCalls {
			if tc.Result == nil {
				continue
			}
			if s := extractStructuralSignals(tc.Result.Output); s != "" {
				signalParts = append(signalParts, s)
			}
		}
	}
	signals := strings.Join(signalParts, "\n")

	var b strings.Builder
	if preferZH {
		b.WriteString("根据已检索到的知识库材料，相关文档如下：\n\n")
	} else {
		b.WriteString("Based on retrieved knowledge-base materials, the related documents are:\n\n")
	}

	if len(titles) == 0 {
		if preferZH {
			b.WriteString("暂未解析出可用的文档标题。")
		} else {
			b.WriteString("No document titles could be recovered.")
		}
	} else {
		for i, title := range titles {
			fmt.Fprintf(&b, "%d. %s\n", i+1, title)
		}
	}

	if signals != "" {
		// Surface category-level counts only — never invent politics prose.
		if preferZH {
			b.WriteString("\n从表格结构中观察到的类别统计（不含具体词条）：\n")
		} else {
			b.WriteString("\nCategory-level stats observed from table structure (no concrete keywords):\n")
		}
		b.WriteString(signals)
		b.WriteByte('\n')
	}
	return b.String()
}

func collectToolResultTitles(state *types.AgentState) []string {
	if state == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var titles []string
	add := func(raw string) {
		title := strings.TrimSpace(raw)
		if title == "" {
			return
		}
		if utf8.RuneCountInString(title) > 200 {
			title = string([]rune(title)[:200]) + "…"
		}
		key := strings.ToLower(title)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		titles = append(titles, title)
	}

	for _, step := range state.RoundSteps {
		for _, tc := range step.ToolCalls {
			if tc.Result == nil {
				continue
			}
			if tc.Result.Data != nil {
				if v, ok := tc.Result.Data["knowledge_title"].(string); ok {
					add(v)
				}
				if v, ok := tc.Result.Data["title"].(string); ok {
					add(v)
				}
			}
			for _, m := range knowledgeTitleAttrRe.FindAllStringSubmatch(tc.Result.Output, -1) {
				if len(m) > 1 {
					add(m[1])
				}
			}
			for _, m := range knowledgeNameTagRe.FindAllStringSubmatch(tc.Result.Output, -1) {
				if len(m) > 1 {
					add(m[1])
				}
			}
		}
	}
	return titles
}
