package types

// CASUserInfo CAS 用户信息（对应接口返回的 data 字段）
type CASUserInfo struct {
	IDStr              string                 `json:"idStr"`              // 用户 ID（字符串）
	ID                 string                 `json:"id"`                 // 用户 ID（数字字符串）
	UnionID            string                 `json:"unionId"`            // Union ID
	LoginName          string                 `json:"loginName"`          // 登录名
	RealName           string                 `json:"realName"`           // 真实姓名
	NickName           string                 `json:"nickName"`           // 昵称
	Email              string                 `json:"email"`              // 邮箱
	MobilePhone        string                 `json:"mobilePhone"`        // 手机号（脱敏）
	Avatar             string                 `json:"image"`              // 头像 URL
	Gender             int                    `json:"gender"`             // 性别（1-男，2-女）
	Province           int                    `json:"province"`           // 省份 ID
	City               int                    `json:"city"`               // 城市 ID
	Country            int                    `json:"country"`            // 国家 ID
	AreaName           string                 `json:"areaName"`           // 地区名称
	NationalIdentifier string                 `json:"nationalIdentifier"` // 身份证号（脱敏）
	Integrity          float64                `json:"integrity"`          // 完整度（CAS 可能返回小数）
	Grade              int                    `json:"grade"`              // 等级
	AuthStatus         map[string]interface{} `json:"authStatus"`         // 认证状态
	PhoneSigned        bool                   `json:"phoneSigned"`        // 手机号是否已认证
}

// 注意：不需要 CASSession 类型定义
// CAS 会话信息存储在浏览器 Cookie 中，不需要数据库存储
// WeKnora 使用 JWT Token（存储在 auth_tokens 表）管理用户认证状态
