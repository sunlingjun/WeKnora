import { get, post, put, del, postUpload } from '@/utils/request'

// TenantRole mirrors internal/types/tenant_member.go's four-role enum.
// Keep the string values aligned with the Go constants.
export type TenantRole = 'owner' | 'admin' | 'contributor' | 'viewer'

export type TenantMemberStatus = 'active' | 'invited' | 'suspended'

// TenantMember is the API projection of a (user, tenant) membership row,
// already joined with the user's email/username/avatar by the backend.
export interface TenantMember {
  user_id: string
  email: string
  username: string
  /** CAS SSO real name when available; preferred for member list display. */
  cas_real_name?: string
  avatar?: string
  role: TenantRole
  status: TenantMemberStatus
  invited_by?: string | null
  joined_at: string
}

export interface ListMembersResponse {
  success: boolean
  data?: {
    members: TenantMember[]
    total: number
    page?: number
    page_size?: number
  }
  message?: string
}

export interface ListMembersParams {
  page?: number
  page_size?: number
  /** 按邮箱/用户名筛选（服务端模糊匹配） */
  q?: string
}

function buildMembersQuery(params: ListMembersParams | undefined): string {
  if (!params) return ''
  const u = new URLSearchParams()
  if (params.page != null && params.page > 0) u.set('page', String(params.page))
  if (params.page_size != null && params.page_size > 0) u.set('page_size', String(params.page_size))
  const q = params.q?.trim()
  if (q) u.set('q', q)
  const qs = u.toString()
  return qs ? `?${qs}` : ''
}

export interface AddMemberRequest {
  email: string
  role: TenantRole
}

export interface AddMemberResponse {
  success: boolean
  data?: TenantMember
  message?: string
}

export interface SimpleResponse {
  success: boolean
  message?: string
}

/**
 * 分页列出空间成员。
 * Backend: GET /api/v1/tenants/:id/members (Viewer+)。查询参数：`q`、`page`、`page_size`。
 */
export async function listMembers(
  tenantId: number,
  params: ListMembersParams = {},
): Promise<ListMembersResponse> {
  const qs = buildMembersQuery(params)
  return (await get(
    `/api/v1/tenants/${tenantId}/members${qs}`,
  )) as unknown as ListMembersResponse
}

/**
 * 遍历分页拉取空间的全部成员（每页最大 100，最多 500 页兜底）。
 * 用于「退出空间」等对全量成员的轻量校验；普通表格请直接使用 {@link listMembers} 分页接口。
 */
export async function fetchAllTenantMembers(tenantId: number): Promise<TenantMember[]> {
  const pageSize = 100
  let page = 1
  const out: TenantMember[] = []
  let total = Number.POSITIVE_INFINITY
  for (let guard = 0; guard < 500 && out.length < total; guard++) {
    const resp = await listMembers(tenantId, { page, page_size: pageSize })
    if (!resp.success || !resp.data) break
    total = resp.data.total
    const batch = resp.data.members || []
    if (batch.length === 0 && page >= 2) break
    out.push(...batch)
    if (batch.length < pageSize) break
    page++
  }
  return out
}

/**
 * Invite an existing user (by email) to the tenant with the given role.
 * Backend: POST /api/v1/tenants/:id/members (Owner+).
 *
 * Returns 404 when the email does not match any registered user — the
 * caller should ask the invitee to register first. PR 3 does not yet
 * support email-based invites for users who don't have an account.
 */
export async function addMember(
  tenantId: number,
  body: AddMemberRequest,
): Promise<AddMemberResponse> {
  return (await post(`/api/v1/tenants/${tenantId}/members`, body)) as unknown as AddMemberResponse
}

/**
 * Change an existing member's role.
 * Backend: PUT /api/v1/tenants/:id/members/:user_id (Owner+).
 *
 * Returns 409 when this would demote the last active Owner of the tenant.
 */
export async function updateMemberRole(
  tenantId: number,
  userId: string,
  role: TenantRole,
): Promise<SimpleResponse> {
  return (await put(`/api/v1/tenants/${tenantId}/members/${userId}`, { role })) as unknown as SimpleResponse
}

/**
 * Remove a member from the tenant.
 * Backend: DELETE /api/v1/tenants/:id/members/:user_id (Owner+).
 *
 * Returns 409 when this would remove the last active Owner.
 */
export async function removeMember(
  tenantId: number,
  userId: string,
): Promise<SimpleResponse> {
  return (await del(`/api/v1/tenants/${tenantId}/members/${userId}`)) as unknown as SimpleResponse
}

/**
 * Quit the tenant on your own. Same last-Owner invariant as
 * removeMember, but does NOT require Owner+ — any active member can
 * call it.
 * Backend: POST /api/v1/tenants/:id/leave (Viewer+).
 */
export async function leaveTenant(tenantId: number): Promise<SimpleResponse> {
  return (await post(`/api/v1/tenants/${tenantId}/leave`)) as unknown as SimpleResponse
}

export type CASImportStatus =
  | 'importable'
  | 'already_member'
  | 'not_found'
  | 'name_mismatch'
  | 'invalid_phone'
  | 'ambiguous'
  | 'local_conflict'
  | 'failed'
  | 'skipped'
  | 'imported'

export type CASImportAction = 'create_user' | 'add_member'

export interface CASImportPreviewRow {
  row: number
  phone_masked: string
  name: string
  cas_user_id?: string
  cas_real_name?: string
  cas_login_name?: string
  weknora_user_id?: string
  weknora_exists: boolean
  already_in_tenant: boolean
  action?: CASImportAction
  status: CASImportStatus
  error?: string
}

export interface CASImportPreview {
  total: number
  importable: number
  will_create: number
  will_add: number
  already_member: number
  not_found: number
  name_mismatch: number
  invalid_phone: number
  ambiguous: number
  local_conflict: number
  failed: number
  role: TenantRole
  rows: CASImportPreviewRow[]
}

export interface CASImportResult {
  total: number
  imported: number
  skipped: number
  failed: number
  role: TenantRole
  rows: CASImportPreviewRow[]
}

export interface CASImportJSONBody {
  phones: string[]
  names?: string[]
  role?: TenantRole
}

const CAS_IMPORT_TIMEOUT_MS = 90000

export interface CASImportAPIResponse<T> {
  success: boolean
  data?: T
  message?: string
}

/**
 * Dry-run 农信用户导入：查用户中心 + 判断知识库账号 / 本空间成员，不写库。
 * Backend: POST /api/v1/tenants/:id/members/cas-import/preview (Owner+)
 */
export async function previewCasImport(
  tenantId: number,
  body: FormData | CASImportJSONBody,
): Promise<CASImportAPIResponse<CASImportPreview>> {
  if (body instanceof FormData) {
    return (await postUpload(
      `/api/v1/tenants/${tenantId}/members/cas-import/preview`,
      body,
      undefined,
      { timeout: CAS_IMPORT_TIMEOUT_MS },
    )) as CASImportAPIResponse<CASImportPreview>
  }
  return (await post(
    `/api/v1/tenants/${tenantId}/members/cas-import/preview`,
    body,
    { timeout: CAS_IMPORT_TIMEOUT_MS },
  )) as unknown as CASImportAPIResponse<CASImportPreview>
}

/**
 * 确认导入农信用户。服务端会重新查询用户中心后再写入。
 * Backend: POST /api/v1/tenants/:id/members/cas-import (Owner+)
 */
export async function confirmCasImport(
  tenantId: number,
  body: FormData | CASImportJSONBody,
): Promise<CASImportAPIResponse<CASImportResult>> {
  if (body instanceof FormData) {
    return (await postUpload(
      `/api/v1/tenants/${tenantId}/members/cas-import`,
      body,
      undefined,
      { timeout: CAS_IMPORT_TIMEOUT_MS },
    )) as CASImportAPIResponse<CASImportResult>
  }
  return (await post(
    `/api/v1/tenants/${tenantId}/members/cas-import`,
    body,
    { timeout: CAS_IMPORT_TIMEOUT_MS },
  )) as unknown as CASImportAPIResponse<CASImportResult>
}
