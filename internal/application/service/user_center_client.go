package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const userCenterRequestTimeout = 8 * time.Second

// userCenterDirectoryClient looks up 农信 users with service credentials
// (systemId + MD5(cert+timestamp)), never with browser cookies.
type userCenterDirectoryClient struct {
	cfg        config.CASUserCenterConfig
	httpClient *http.Client
}

// NewUserCenterDirectoryClient builds a directory client from cas.user_center
// plus env overrides (CAS_UC_URL / CAS_UC_SYSTEM_ID / CAS_UC_CERT).
func NewUserCenterDirectoryClient(cfg *config.Config) interfaces.UserCenterDirectory {
	var uc config.CASUserCenterConfig
	if cfg != nil && cfg.CAS != nil {
		uc = cfg.CAS.ResolveUserCenter()
	}
	return &userCenterDirectoryClient{
		cfg: uc,
		httpClient: &http.Client{
			Timeout: userCenterRequestTimeout,
		},
	}
}

func (c *userCenterDirectoryClient) Configured() bool {
	return c != nil && c.cfg.Configured()
}

func (c *userCenterDirectoryClient) HasBaseURL() bool {
	return c != nil && strings.TrimSpace(c.cfg.URL) != ""
}

func (c *userCenterDirectoryClient) FindByAuthorizedPhone(ctx context.Context, phone string) (*types.CASUserInfo, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("user center is not configured")
	}
	body, err := c.postForm(ctx, "person/findByAuthorizedPhone", url.Values{"phone": {phone}})
	if err != nil {
		return nil, err
	}
	info, err := parseUserArchiveObject(body)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (c *userCenterDirectoryClient) SearchByNameOrPhone(ctx context.Context, keyword string) ([]*types.CASUserInfo, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("user center is not configured")
	}
	body, err := c.postForm(ctx, "person/searchByNameOrPhone", url.Values{"keyword": {keyword}})
	if err != nil {
		return nil, err
	}
	return parseUserArchiveList(body)
}

func (c *userCenterDirectoryClient) GetBoIDByZNTToken(ctx context.Context, token string) (string, error) {
	if !c.HasBaseURL() {
		return "", fmt.Errorf("user center url is not configured")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("empty znt token")
	}
	path := "login/get-boId-by-znt-token/" + url.PathEscape(token)
	raw, err := c.getNoAuth(ctx, path)
	if err != nil {
		return "", err
	}
	return parseBoIDFromDataField(raw)
}

func (c *userCenterDirectoryClient) GetBoIDByUcTicket(ctx context.Context, ticket string) (string, error) {
	if !c.HasBaseURL() {
		return "", fmt.Errorf("user center url is not configured")
	}
	ticket = strings.TrimSpace(ticket)
	if ticket == "" {
		return "", fmt.Errorf("empty uc ticket")
	}
	raw, err := c.postFormNoAuth(ctx, "login/getUserByUcTicket", url.Values{"ticket": {ticket}})
	if err != nil {
		return "", err
	}
	return parseBoIDFromUcTicket(raw)
}

func (c *userCenterDirectoryClient) GetUserArchive(ctx context.Context, boID string) (*types.CASUserInfo, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("user center is not configured")
	}
	boID = strings.TrimSpace(boID)
	if boID == "" {
		return nil, fmt.Errorf("empty bo id")
	}
	raw, err := c.postForm(ctx, "person/getUserArchive/"+url.PathEscape(boID), url.Values{})
	if err != nil {
		return nil, err
	}
	return parseUserArchiveObjectStrict(raw, boID)
}

func (c *userCenterDirectoryClient) postForm(ctx context.Context, path string, form url.Values) ([]byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, userCenterRequestTimeout)
	defer cancel()

	endpoint := joinUserCenterURL(c.cfg.URL, path)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("user center request: %w", err)
	}
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("systemId", c.cfg.SystemID)
	req.Header.Set("timestamp", ts)
	req.Header.Set("accessToken", userCenterAccessToken(c.cfg.Cert, ts))

	return c.doUserCenterRequest(ctx, req, path)
}

func (c *userCenterDirectoryClient) getNoAuth(ctx context.Context, path string) ([]byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, userCenterRequestTimeout)
	defer cancel()

	endpoint := joinUserCenterURL(c.cfg.URL, path)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("user center request: %w", err)
	}
	return c.doUserCenterRequest(ctx, req, path)
}

func (c *userCenterDirectoryClient) postFormNoAuth(ctx context.Context, path string, form url.Values) ([]byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, userCenterRequestTimeout)
	defer cancel()

	endpoint := joinUserCenterURL(c.cfg.URL, path)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("user center request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.doUserCenterRequest(ctx, req, path)
}

func (c *userCenterDirectoryClient) doUserCenterRequest(ctx context.Context, req *http.Request, path string) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user center call %s: %w", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("user center read %s: %w", path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Warnf(ctx, "user center %s http=%d body_len=%d", path, resp.StatusCode, len(raw))
		return nil, fmt.Errorf("user center %s http %d", path, resp.StatusCode)
	}
	return raw, nil
}

func userCenterAccessToken(cert, timestamp string) string {
	sum := md5.Sum([]byte(cert + timestamp))
	return hex.EncodeToString(sum[:])
}

func joinUserCenterURL(base, path string) string {
	return strings.TrimRight(strings.TrimSpace(base), "/") + "/" + strings.TrimLeft(path, "/")
}

type userCenterEnvelope struct {
	Code json.RawMessage `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type userArchiveDTO struct {
	ID          json.RawMessage `json:"id"`
	IDStr       string          `json:"idStr"`
	UnionID     string          `json:"unionId"`
	LoginName   string          `json:"loginName"`
	RealName    string          `json:"realName"`
	NickName    string          `json:"nickName"`
	Email       string          `json:"email"`
	MobilePhone string          `json:"mobilePhone"`
	Avatar      string          `json:"image"`
	PhoneSigned bool            `json:"phoneSigned"`
}

func parseUserArchiveObject(raw []byte) (*types.CASUserInfo, error) {
	env, err := decodeUserCenterEnvelope(raw)
	if err != nil {
		return nil, err
	}
	if !userCenterCodeOK(env.Code) || isJSONNull(env.Data) {
		return nil, nil
	}
	var dto userArchiveDTO
	if err := json.Unmarshal(env.Data, &dto); err != nil {
		return nil, fmt.Errorf("user center person payload: %w", err)
	}
	return archiveToCASUserInfo(dto), nil
}

// parseUserArchiveObjectStrict treats non-zero UC code as an error (auth path),
// unlike parseUserArchiveObject which soft-misses for directory lookup.
func parseUserArchiveObjectStrict(raw []byte, boID string) (*types.CASUserInfo, error) {
	env, err := decodeUserCenterEnvelope(raw)
	if err != nil {
		return nil, err
	}
	if !userCenterCodeOK(env.Code) {
		return nil, fmt.Errorf("user center getUserArchive code=%s", strings.TrimSpace(string(env.Code)))
	}
	if isJSONNull(env.Data) {
		return nil, fmt.Errorf("user archive empty for boId=%s", boID)
	}
	var dto userArchiveDTO
	if err := json.Unmarshal(env.Data, &dto); err != nil {
		return nil, fmt.Errorf("user center person payload: %w", err)
	}
	info := archiveToCASUserInfo(dto)
	if info == nil || info.ID == "" {
		return nil, fmt.Errorf("user archive empty for boId=%s", boID)
	}
	return info, nil
}

func parseUserArchiveList(raw []byte) ([]*types.CASUserInfo, error) {
	env, err := decodeUserCenterEnvelope(raw)
	if err != nil {
		return nil, err
	}
	if !userCenterCodeOK(env.Code) || isJSONNull(env.Data) {
		return nil, nil
	}
	var list []userArchiveDTO
	if err := json.Unmarshal(env.Data, &list); err != nil {
		var wrapped struct {
			List    []userArchiveDTO `json:"list"`
			Records []userArchiveDTO `json:"records"`
			Items   []userArchiveDTO `json:"items"`
		}
		if wrapErr := json.Unmarshal(env.Data, &wrapped); wrapErr != nil {
			return nil, fmt.Errorf("user center search payload: %w", err)
		}
		list = wrapped.List
		if len(list) == 0 {
			list = wrapped.Records
		}
		if len(list) == 0 {
			list = wrapped.Items
		}
	}
	out := make([]*types.CASUserInfo, 0, len(list))
	for _, dto := range list {
		if info := archiveToCASUserInfo(dto); info != nil && info.ID != "" {
			out = append(out, info)
		}
	}
	return out, nil
}

func decodeUserCenterEnvelope(raw []byte) (userCenterEnvelope, error) {
	var env userCenterEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return env, fmt.Errorf("user center json: %w", err)
	}
	return env, nil
}

func userCenterCodeOK(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	switch s {
	case "", "0", `"0"`, "200", `"200"`:
		return true
	default:
		return false
	}
}

func isJSONNull(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "" || s == "null"
}

func archiveToCASUserInfo(dto userArchiveDTO) *types.CASUserInfo {
	id := strings.TrimSpace(dto.IDStr)
	if id == "" {
		id = stringifyJSONID(dto.ID)
	}
	return &types.CASUserInfo{
		ID:          id,
		IDStr:       id,
		UnionID:     strings.TrimSpace(dto.UnionID),
		LoginName:   strings.TrimSpace(dto.LoginName),
		RealName:    strings.TrimSpace(dto.RealName),
		NickName:    strings.TrimSpace(dto.NickName),
		Email:       strings.TrimSpace(dto.Email),
		MobilePhone: strings.TrimSpace(dto.MobilePhone),
		Avatar:      strings.TrimSpace(dto.Avatar),
		PhoneSigned: dto.PhoneSigned,
	}
}

func stringifyJSONID(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	if strings.HasPrefix(s, `"`) {
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			return strings.TrimSpace(str)
		}
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return strings.TrimSpace(n.String())
	}
	return strings.Trim(s, `"`)
}

func parseBoIDFromDataField(raw []byte) (string, error) {
	env, err := decodeUserCenterEnvelope(raw)
	if err != nil {
		return "", err
	}
	if !userCenterCodeOK(env.Code) {
		return "", fmt.Errorf("user center code %s", strings.TrimSpace(string(env.Code)))
	}
	id := stringifyJSONID(env.Data)
	if id == "" {
		return "", fmt.Errorf("user center empty bo id")
	}
	return id, nil
}

func parseBoIDFromUcTicket(raw []byte) (string, error) {
	env, err := decodeUserCenterEnvelope(raw)
	if err != nil {
		return "", err
	}
	if !userCenterCodeOK(env.Code) {
		return "", fmt.Errorf("user center code %s", strings.TrimSpace(string(env.Code)))
	}
	if isJSONNull(env.Data) {
		return "", fmt.Errorf("user center empty bo id")
	}
	var payload struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return "", fmt.Errorf("user center uc ticket payload: %w", err)
	}
	id := stringifyJSONID(payload.ID)
	if id == "" {
		return "", fmt.Errorf("user center empty bo id")
	}
	return id, nil
}
