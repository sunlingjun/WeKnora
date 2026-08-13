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
