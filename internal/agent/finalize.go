package agent

import (
	"context"
	"fmt"
	"time"

	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/common"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/searchutil"
	"github.com/Tencent/WeKnora/internal/types"
)

const (
	// finalAnswerToolResultCap limits each tool payload fed into final-answer
	// synthesis so a prior deep-read of large tables cannot trip provider
	// content-policy refusals a second time.
	finalAnswerToolResultCap = 4000
)

func finalAnswerImageRequirement(hasRetrievedImage bool) string {
	if !hasRetrievedImage {
		return ""
	}
	return `
5. Retrieved tool results contain Markdown images. Unless the user explicitly requested text-only output or every image is clearly unrelated, the final answer MUST include at least one relevant Markdown image copied verbatim from the tool results. Preserve its complete URL exactly. Use ASCII half-width parentheses exactly as ![alt](url) and never use full-width （ or ）. Place the image immediately after the paragraph it supports. When multiple images support different sections, distribute them across those sections instead of stopping after the first image.
6. Before finishing, silently verify that the answer contains a Markdown image when requirement 5 applies.`
}

// streamFinalAnswerToEventBus streams the final answer generation through EventBus
func (e *AgentEngine) streamFinalAnswerToEventBus(
	ctx context.Context,
	query string,
	state *types.AgentState,
	sessionID string,
) error {
	totalToolCalls := countTotalToolCalls(state.RoundSteps)
	logger.Infof(ctx, "[Agent][FinalAnswer] Synthesizing from %d steps, %d tool calls",
		len(state.RoundSteps), totalToolCalls)
	common.PipelineInfo(ctx, "Agent", "final_answer_start", map[string]interface{}{
		"session_id":   sessionID,
		"query":        query,
		"steps":        len(state.RoundSteps),
		"tool_results": totalToolCalls,
	})

	messages := e.buildFinalAnswerMessages(ctx, sessionID, query, state)
	answerID := generateEventID("answer")
	logger.Debugf(ctx, "[Agent][FinalAnswer] AnswerID: %s", answerID)

	fullAnswer, err := e.streamFinalAnswerMessages(ctx, sessionID, answerID, messages)
	if err != nil {
		if isContentPolicyError(err) {
			logger.Warnf(ctx, "[Agent][FinalAnswer] Content policy blocked synthesis; recovering with safe answer: %v", err)
			common.PipelineWarn(ctx, "Agent", "final_answer_content_policy", map[string]interface{}{
				"session_id": sessionID,
				"error":      err.Error(),
			})
			// Close the failed stream so UI does not leave a hanging answer ID,
			// then replace any partial chunks with the safe recovery answer.
			e.eventBus.Emit(ctx, event.Event{
				ID:        answerID,
				Type:      event.EventAgentFinalAnswer,
				SessionID: sessionID,
				Data: event.AgentFinalAnswerData{
					Content: "",
					Done:    true,
				},
			})
			e.recoverFromContentPolicy(ctx, query, state, sessionID)
			return nil
		}
		logger.Errorf(ctx, "[Agent][FinalAnswer] Final answer generation failed: %v", err)
		common.PipelineError(ctx, "Agent", "final_answer_stream_failed", map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return err
	}

	logger.Infof(ctx, "[Agent][FinalAnswer] Final answer generated: %d characters", len(fullAnswer))
	common.PipelineInfo(ctx, "Agent", "final_answer_done", map[string]interface{}{
		"session_id": sessionID,
		"answer_len": len(fullAnswer),
	})
	state.FinalAnswer = fullAnswer
	return nil
}

func (e *AgentEngine) buildFinalAnswerMessages(
	ctx context.Context,
	sessionID, query string,
	state *types.AgentState,
) []chat.Message {
	systemPrompt := e.buildSystemPrompt(ctx)
	userTurn := e.RenderUserTurnContent(sessionID, query)

	messages := []chat.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userTurn},
	}

	hasRetrievedImage := false
	for _, step := range state.RoundSteps {
		for _, toolCall := range step.ToolCalls {
			if toolCall.Result == nil {
				continue
			}
			if searchutil.MarkdownImageRegex.MatchString(toolCall.Result.Output) {
				hasRetrievedImage = true
			}
			modelOutput := e.modelContext.ModelToolResultForTool(toolCall.Name, toolCall.Result)
			modelOutput = agenttools.TruncateToolOutput(modelOutput, finalAnswerToolResultCap)
			messages = append(messages, chat.Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool %s returned: %s", toolCall.Name, modelOutput),
			})
		}
	}

	imageRequirement := finalAnswerImageRequirement(hasRetrievedImage)

	finalPrompt := fmt.Sprintf(`Based on the above tool call results, generate a complete answer for the user's question.

User question: %s

Requirements:
1. Answer based on the actually retrieved content
2. Organize the answer in a structured format
3. If information is insufficient, honestly state so
4. IMPORTANT: Respond in the same language as the user's question
%s

Now generate the final answer:`, query, imageRequirement)

	return append(messages, chat.Message{Role: "user", Content: finalPrompt})
}

func (e *AgentEngine) streamFinalAnswerMessages(
	ctx context.Context,
	sessionID, answerID string,
	messages []chat.Message,
) (string, error) {
	answerDoneEmitted := false
	llmResult, err := e.streamLLMToEventBus(
		ctx,
		messages,
		&chat.ChatOptions{Temperature: e.config.Temperature}, // Thinking disabled for final answer synthesis
		func(chunk *types.StreamResponse, fullContent string) {
			if chunk.ResponseType == types.ResponseTypeThinking {
				return
			}
			if chunk.Content != "" {
				logger.Debugf(ctx, "[Agent][FinalAnswer] Emitting answer chunk: %d chars", len(chunk.Content))
				e.eventBus.Emit(ctx, event.Event{
					ID:        answerID,
					Type:      event.EventAgentFinalAnswer,
					SessionID: sessionID,
					Data: event.AgentFinalAnswerData{
						Content: chunk.Content,
						Done:    chunk.Done,
					},
				})
				if chunk.Done {
					answerDoneEmitted = true
				}
			}
		},
	)
	if err != nil {
		return "", err
	}

	if !answerDoneEmitted {
		e.eventBus.Emit(ctx, event.Event{
			ID:        answerID,
			Type:      event.EventAgentFinalAnswer,
			SessionID: sessionID,
			Data: event.AgentFinalAnswerData{
				Content: "",
				Done:    true,
			},
		})
	}

	return agenttools.StripThinkBlocks(llmResult.Content), nil
}

// handleMaxIterations generates a final answer when the agent loop exhausted all iterations
// without the LLM producing a natural stop. It marks state.IsComplete = true.
func (e *AgentEngine) handleMaxIterations(
	ctx context.Context, query string, state *types.AgentState, sessionID string,
) {
	logger.Info(ctx, "Reached max iterations, generating final answer")
	common.PipelineWarn(ctx, "Agent", "max_iterations_reached", map[string]interface{}{
		"iterations": state.CurrentRound,
		"max":        e.config.MaxIterations,
	})

	if err := e.streamFinalAnswerToEventBus(ctx, query, state, sessionID); err != nil {
		// Content-policy refusals are already recovered inside
		// streamFinalAnswerToEventBus (returns nil); any remaining err is fatal.
		logger.Errorf(ctx, "Failed to synthesize final answer: %v", err)
		common.PipelineError(ctx, "Agent", "final_answer_failed", map[string]interface{}{
			"error": err.Error(),
		})
		state.FinalAnswer = "Sorry, I was unable to generate a complete answer."
	}
	state.IsComplete = true
}

// emitCompletionEvent emits the EventAgentComplete event with execution summary.
func (e *AgentEngine) emitCompletionEvent(
	ctx context.Context, state *types.AgentState, sessionID, messageID string, startTime time.Time,
) {
	knowledgeRefsInterface := make([]interface{}, 0, len(state.KnowledgeRefs))
	for _, ref := range state.KnowledgeRefs {
		knowledgeRefsInterface = append(knowledgeRefsInterface, ref)
	}

	e.eventBus.Emit(ctx, event.Event{
		ID:        generateEventID("complete"),
		Type:      event.EventAgentComplete,
		SessionID: sessionID,
		Data: event.AgentCompleteData{
			FinalAnswer:     state.FinalAnswer,
			KnowledgeRefs:   knowledgeRefsInterface,
			AgentSteps:      state.RoundSteps,
			TotalSteps:      len(state.RoundSteps),
			TotalDurationMs: time.Since(startTime).Milliseconds(),
			MessageID:       messageID,
		},
	})

	logger.Infof(ctx, "Agent execution completed in %d rounds", state.CurrentRound)
}
