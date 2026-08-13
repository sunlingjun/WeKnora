package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/xuri/excelize/v2"
)

const (
	casImportConcurrency    = 5
	casImportOverallTimeout = 60 * time.Second
	casImportMaxHeaderScan  = 15
)

var (
	ErrCASImportNotConfigured = errors.New("农信用户导入未配置")
	ErrCASImportTooManyRows   = errors.New("单次最多导入 200 行")
	ErrCASImportEmpty         = errors.New("没有可导入的手机号")
	ErrCASImportInvalidRole   = errors.New("role must be admin, contributor or viewer")
	ErrCASImportOwnerRole     = errors.New("cannot import members as owner")
	ErrCASImportUnsupported   = errors.New("仅支持 xlsx 或 csv")
)

type casImportLastActiveSetter interface {
	UpdateUserPreferences(ctx context.Context, userID string, patch types.UserPreferences) (types.UserPreferences, error)
}

type casMemberImportService struct {
	dir           interfaces.UserCenterDirectory
	userRepo      interfaces.UserRepository
	casAuth       interfaces.CASAuthService
	memberService interfaces.TenantMemberService
	userService   casImportLastActiveSetter
}

// NewCASMemberImportService wires preview/import against the 农信 directory.
func NewCASMemberImportService(
	dir interfaces.UserCenterDirectory,
	userRepo interfaces.UserRepository,
	casAuth interfaces.CASAuthService,
	memberService interfaces.TenantMemberService,
	userService interfaces.UserService,
) interfaces.CASMemberImportService {
	return &casMemberImportService{
		dir:           dir,
		userRepo:      userRepo,
		casAuth:       casAuth,
		memberService: memberService,
		userService:   userService,
	}
}

func (s *casMemberImportService) Configured() bool {
	return s != nil && s.dir != nil && s.dir.Configured()
}

func (s *casMemberImportService) ParsePhonesText(text string) []types.CASImportRow {
	text = strings.TrimSpace(strings.TrimPrefix(text, "\ufeff"))
	if text == "" {
		return nil
	}
	replacer := strings.NewReplacer("\r\n", "\n", "\r", "\n", ";", "\n", "，", "\n")
	lines := strings.Split(replacer.Replace(text), "\n")
	out := make([]types.CASImportRow, 0, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		phone, name := splitPhoneNameLine(line)
		if phone == "" && name == "" {
			continue
		}
		out = append(out, types.CASImportRow{Row: i + 1, Phone: phone, Name: name})
	}
	return out
}

func splitPhoneNameLine(line string) (phone, name string) {
	for _, sep := range []string{",", "\t", " "} {
		if strings.Contains(line, sep) {
			parts := strings.SplitN(line, sep, 2)
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
	}
	return strings.TrimSpace(line), ""
}

func (s *casMemberImportService) ParseFile(filename string, r io.Reader) ([]types.CASImportRow, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	raw, err := io.ReadAll(io.LimitReader(r, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read import file: %w", err)
	}
	switch ext {
	case ".xlsx", ".xlsm":
		return parseXLSXRows(raw)
	case ".csv":
		return parseCSVRows(raw)
	default:
		// Some browsers send a file without extension; sniff ZIP (xlsx) vs text.
		if len(raw) >= 2 && raw[0] == 'P' && raw[1] == 'K' {
			return parseXLSXRows(raw)
		}
		if ext == "" || ext == ".txt" {
			return parseCSVRows(raw)
		}
		return nil, ErrCASImportUnsupported
	}
}

func parseXLSXRows(raw []byte) ([]types.CASImportRow, error) {
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer func() { _ = f.Close() }()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, ErrCASImportEmpty
	}
	grid, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("read xlsx sheet: %w", err)
	}
	return rowsFromGrid(grid)
}

func parseCSVRows(raw []byte) ([]types.CASImportRow, error) {
	text := strings.TrimPrefix(string(raw), "\ufeff")
	cr := csv.NewReader(strings.NewReader(text))
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1
	grid, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	return rowsFromGrid(grid)
}

func rowsFromGrid(grid [][]string) ([]types.CASImportRow, error) {
	headerIdx, phoneCol, nameCol := findImportHeader(grid)
	if headerIdx < 0 || phoneCol < 0 {
		return nil, fmt.Errorf("未找到「手机号」列")
	}
	out := make([]types.CASImportRow, 0, len(grid)-headerIdx)
	for i := headerIdx + 1; i < len(grid); i++ {
		row := grid[i]
		phone := cellAt(row, phoneCol)
		name := ""
		if nameCol >= 0 {
			name = cellAt(row, nameCol)
		}
		if strings.TrimSpace(phone) == "" && strings.TrimSpace(name) == "" {
			continue
		}
		out = append(out, types.CASImportRow{
			Row:   i + 1,
			Phone: phone,
			Name:  name,
		})
	}
	return out, nil
}

func cellAt(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func findImportHeader(grid [][]string) (headerIdx, phoneCol, nameCol int) {
	limit := len(grid)
	if limit > casImportMaxHeaderScan {
		limit = casImportMaxHeaderScan
	}
	for i := 0; i < limit; i++ {
		p, n := -1, -1
		for j, cell := range grid[i] {
			key := normalizeHeader(cell)
			if p < 0 && isPhoneHeader(key) {
				p = j
			}
			if n < 0 && isNameHeader(key) {
				n = j
			}
		}
		if p >= 0 {
			return i, p, n
		}
	}
	return -1, -1, -1
}

func normalizeHeader(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

func isPhoneHeader(key string) bool {
	switch key {
	case "手机号", "手机", "电话", "联系电话", "phone", "cellphone", "mobile", "mobilephone":
		return true
	default:
		return false
	}
}

func isNameHeader(key string) bool {
	switch key {
	case "姓名", "名字", "name", "realname", "username":
		return true
	default:
		return false
	}
}

func (s *casMemberImportService) Preview(ctx context.Context, tenantID uint64, rows []types.CASImportRow) (*types.CASImportPreview, error) {
	if err := s.guard(rows); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, casImportOverallTimeout)
	defer cancel()
	classified := s.classifyAll(ctx, tenantID, rows)
	out := &types.CASImportPreview{
		Total: len(classified),
		Rows:  make([]types.CASImportPreviewRow, 0, len(classified)),
	}
	for _, item := range classified {
		out.Rows = append(out.Rows, item.out)
		tallyPreview(out, item.out)
	}
	return out, nil
}

func (s *casMemberImportService) Import(
	ctx context.Context,
	tenantID uint64,
	role types.TenantRole,
	invitedBy *string,
	rows []types.CASImportRow,
) (*types.CASImportResult, error) {
	if err := s.guard(rows); err != nil {
		return nil, err
	}
	if err := validateImportRole(role); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, casImportOverallTimeout)
	defer cancel()
	classified := s.classifyAll(ctx, tenantID, rows)
	result := &types.CASImportResult{
		Total: len(classified),
		Role:  role,
		Rows:  make([]types.CASImportPreviewRow, 0, len(classified)),
	}
	for _, item := range classified {
		row := s.applyImport(ctx, tenantID, role, invitedBy, item)
		switch row.Status {
		case types.CASImportStatusImported:
			result.Imported++
		case types.CASImportStatusSkipped, types.CASImportStatusAlreadyMember:
			result.Skipped++
			row.Status = types.CASImportStatusSkipped
		default:
			result.Failed++
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}

func (s *casMemberImportService) guard(rows []types.CASImportRow) error {
	if !s.Configured() {
		return ErrCASImportNotConfigured
	}
	if len(rows) == 0 {
		return ErrCASImportEmpty
	}
	if len(rows) > types.CASImportMaxRows {
		return ErrCASImportTooManyRows
	}
	return nil
}

func validateImportRole(role types.TenantRole) error {
	if role == types.TenantRoleOwner {
		return ErrCASImportOwnerRole
	}
	if role != types.TenantRoleAdmin && role != types.TenantRoleContributor && role != types.TenantRoleViewer {
		return ErrCASImportInvalidRole
	}
	return nil
}

type classifiedCASRow struct {
	out types.CASImportPreviewRow
	cas *types.CASUserInfo
}

func (s *casMemberImportService) classifyAll(ctx context.Context, tenantID uint64, rows []types.CASImportRow) []classifiedCASRow {
	out := make([]classifiedCASRow, len(rows))
	sem := make(chan struct{}, casImportConcurrency)
	var wg sync.WaitGroup
	for i, row := range rows {
		wg.Add(1)
		go func(i int, row types.CASImportRow) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = s.classifyRow(ctx, tenantID, row)
		}(i, row)
	}
	wg.Wait()
	return out
}

func (s *casMemberImportService) classifyRow(ctx context.Context, tenantID uint64, row types.CASImportRow) classifiedCASRow {
	preview := types.CASImportPreviewRow{
		Row:  row.Row,
		Name: strings.TrimSpace(row.Name),
	}
	phone, ok := normalizeCNMobile(row.Phone)
	preview.PhoneMasked = maskCNMobile(phone)
	if !ok {
		preview.PhoneMasked = maskCNMobile(digitsOnly(row.Phone))
		preview.Status = types.CASImportStatusInvalidPhone
		preview.Error = "invalid phone"
		return classifiedCASRow{out: preview}
	}

	cas, status, errMsg := s.lookupCAS(ctx, phone, preview.Name)
	if status != "" {
		preview.Status = status
		preview.Error = errMsg
		if cas != nil {
			fillCASPreview(&preview, cas)
		}
		return classifiedCASRow{out: preview, cas: cas}
	}
	fillCASPreview(&preview, cas)
	if strings.TrimSpace(cas.ID) == "" {
		preview.Status = types.CASImportStatusFailed
		preview.Error = "missing cas_user_id"
		return classifiedCASRow{out: preview, cas: cas}
	}

	local, conflict, err := s.lookupLocalUser(ctx, cas)
	if err != nil {
		preview.Status = types.CASImportStatusFailed
		preview.Error = err.Error()
		return classifiedCASRow{out: preview, cas: cas}
	}
	if conflict {
		preview.Status = types.CASImportStatusLocalConflict
		preview.Error = "email bound to another cas_user_id"
		if local != nil {
			preview.WeKnoraUserID = local.ID
			preview.WeKnoraExists = true
		}
		return classifiedCASRow{out: preview, cas: cas}
	}
	if local == nil {
		preview.WeKnoraExists = false
		preview.Action = types.CASImportActionCreateUser
		preview.Status = types.CASImportStatusImportable
		return classifiedCASRow{out: preview, cas: cas}
	}

	preview.WeKnoraExists = true
	preview.WeKnoraUserID = local.ID
	member, err := s.memberService.GetMembership(ctx, local.ID, tenantID)
	if err != nil {
		preview.Status = types.CASImportStatusFailed
		preview.Error = err.Error()
		return classifiedCASRow{out: preview, cas: cas}
	}
	if member != nil && member.Status == types.TenantMemberStatusActive {
		preview.AlreadyInTenant = true
		preview.Status = types.CASImportStatusAlreadyMember
		return classifiedCASRow{out: preview, cas: cas}
	}
	preview.Action = types.CASImportActionAddMember
	preview.Status = types.CASImportStatusImportable
	return classifiedCASRow{out: preview, cas: cas}
}

func fillCASPreview(preview *types.CASImportPreviewRow, cas *types.CASUserInfo) {
	if preview == nil || cas == nil {
		return
	}
	preview.CASUserID = cas.ID
	preview.CASRealName = cas.RealName
	preview.CASLoginName = cas.LoginName
}

func (s *casMemberImportService) lookupCAS(ctx context.Context, phone, excelName string) (*types.CASUserInfo, string, string) {
	info, err := s.dir.FindByAuthorizedPhone(ctx, phone)
	if err != nil {
		logger.Warnf(ctx, "user center findByAuthorizedPhone phone=%s err=%v", maskCNMobile(phone), err)
		return nil, types.CASImportStatusFailed, "user center lookup failed"
	}
	if info != nil && strings.TrimSpace(info.ID) != "" {
		if excelName != "" && !casNamesEqual(excelName, info.RealName) {
			return info, types.CASImportStatusNameMismatch, "name mismatch"
		}
		return info, "", ""
	}

	list, err := s.dir.SearchByNameOrPhone(ctx, phone)
	if err != nil {
		logger.Warnf(ctx, "user center searchByNameOrPhone phone=%s err=%v", maskCNMobile(phone), err)
		return nil, types.CASImportStatusFailed, "user center search failed"
	}
	picked, status := pickCASFromSearch(list, excelName)
	if status != "" {
		return picked, status, status
	}
	if excelName != "" && picked != nil && !casNamesEqual(excelName, picked.RealName) {
		return picked, types.CASImportStatusNameMismatch, "name mismatch"
	}
	return picked, "", ""
}

func pickCASFromSearch(list []*types.CASUserInfo, excelName string) (*types.CASUserInfo, string) {
	filtered := make([]*types.CASUserInfo, 0, len(list))
	for _, item := range list {
		if item == nil || strings.TrimSpace(item.ID) == "" {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		return nil, types.CASImportStatusNotFound
	}
	if len(filtered) == 1 {
		return filtered[0], ""
	}
	if strings.TrimSpace(excelName) == "" {
		return nil, types.CASImportStatusAmbiguous
	}
	matched := make([]*types.CASUserInfo, 0)
	for _, item := range filtered {
		if casNamesEqual(excelName, item.RealName) {
			matched = append(matched, item)
		}
	}
	if len(matched) == 1 {
		return matched[0], ""
	}
	if len(matched) == 0 {
		return nil, types.CASImportStatusNotFound
	}
	return nil, types.CASImportStatusAmbiguous
}

func (s *casMemberImportService) lookupLocalUser(ctx context.Context, cas *types.CASUserInfo) (*types.User, bool, error) {
	if cas == nil {
		return nil, false, nil
	}
	if cas.ID != "" {
		user, err := s.userRepo.GetUserByCASUserID(ctx, cas.ID)
		if err == nil && user != nil {
			return user, false, nil
		}
		if err != nil && !errors.Is(err, apprepo.ErrUserNotFound) {
			return nil, false, err
		}
	}
	if strings.TrimSpace(cas.Email) == "" {
		return nil, false, nil
	}
	user, err := s.userRepo.GetUserByEmail(ctx, cas.Email)
	if err != nil {
		if errors.Is(err, apprepo.ErrUserNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if user == nil {
		return nil, false, nil
	}
	if user.CASUserID != "" && user.CASUserID != cas.ID {
		return user, true, nil
	}
	return user, false, nil
}

func (s *casMemberImportService) applyImport(
	ctx context.Context,
	tenantID uint64,
	role types.TenantRole,
	invitedBy *string,
	item classifiedCASRow,
) types.CASImportPreviewRow {
	row := item.out
	if row.Status == types.CASImportStatusAlreadyMember {
		row.Status = types.CASImportStatusSkipped
		return row
	}
	if row.Status != types.CASImportStatusImportable {
		if row.Status == "" {
			row.Status = types.CASImportStatusFailed
		}
		return row
	}
	if item.cas == nil || strings.TrimSpace(item.cas.ID) == "" {
		row.Status = types.CASImportStatusFailed
		row.Error = "missing cas_user_id"
		row.Action = ""
		return row
	}

	user, err := s.casAuth.AutoBindUser(ctx, item.cas)
	if err != nil {
		row.Status = types.CASImportStatusFailed
		row.Error = err.Error()
		return row
	}
	if user == nil || user.ID == "" {
		row.Status = types.CASImportStatusFailed
		row.Error = "AutoBindUser returned empty user"
		return row
	}
	if _, err := s.casAuth.AutoBindTenant(ctx, item.cas, user); err != nil {
		row.Status = types.CASImportStatusFailed
		row.Error = err.Error()
		return row
	}
	if _, err := s.memberService.AddMember(ctx, user.ID, tenantID, role, invitedBy); err != nil {
		if errors.Is(err, ErrMembershipAlreadyExists) {
			row.Status = types.CASImportStatusSkipped
			row.AlreadyInTenant = true
			row.WeKnoraExists = true
			row.WeKnoraUserID = user.ID
			return row
		}
		row.Status = types.CASImportStatusFailed
		row.Error = err.Error()
		return row
	}
	row.Status = types.CASImportStatusImported
	row.WeKnoraUserID = user.ID
	row.WeKnoraExists = true
	row.AlreadyInTenant = true
	row.Error = ""
	s.setLastActive(ctx, user.ID, tenantID)
	logger.Infof(ctx, "CAS member imported phone=%s cas_user_id=%s user=%s tenant=%d action=%s",
		row.PhoneMasked, row.CASUserID, user.ID, tenantID, row.Action)
	return row
}

func (s *casMemberImportService) setLastActive(ctx context.Context, userID string, tenantID uint64) {
	if s.userService == nil {
		return
	}
	tid := tenantID
	if _, err := s.userService.UpdateUserPreferences(ctx, userID, types.UserPreferences{
		LastActiveTenantID: &tid,
	}); err != nil {
		logger.Warnf(ctx, "set last_active after CAS import user=%s tenant=%d err=%v", userID, tenantID, err)
	}
}

func tallyPreview(out *types.CASImportPreview, row types.CASImportPreviewRow) {
	switch row.Status {
	case types.CASImportStatusImportable:
		out.Importable++
		if row.Action == types.CASImportActionCreateUser {
			out.WillCreate++
		} else if row.Action == types.CASImportActionAddMember {
			out.WillAdd++
		}
	case types.CASImportStatusAlreadyMember:
		out.AlreadyMember++
	case types.CASImportStatusNotFound:
		out.NotFound++
	case types.CASImportStatusNameMismatch:
		out.NameMismatch++
	case types.CASImportStatusInvalidPhone:
		out.InvalidPhone++
	case types.CASImportStatusAmbiguous:
		out.Ambiguous++
	case types.CASImportStatusLocalConflict:
		out.LocalConflict++
	default:
		out.Failed++
	}
}

func normalizeCNMobile(raw string) (string, bool) {
	digits := digitsOnly(raw)
	if strings.HasPrefix(digits, "86") && len(digits) == 13 {
		digits = digits[2:]
	}
	if len(digits) != 11 || digits[0] != '1' {
		return digits, false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return digits, false
		}
	}
	return digits, true
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func maskCNMobile(phone string) string {
	if phone == "" {
		return ""
	}
	if len(phone) >= 11 {
		return phone[:3] + "****" + phone[len(phone)-4:]
	}
	if len(phone) >= 7 {
		return phone[:3] + "****" + phone[len(phone)-4:]
	}
	return "****"
}

func casNamesEqual(a, b string) bool {
	return compactName(a) == compactName(b)
}

func compactName(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
}
