/**
 * 知识库权限检查工具
 * 统一处理直接共享知识库的权限判断逻辑
 */

/** 知识库基本信息（用于权限判断） */
export interface KBPermissionInfo {
  visibility?: 'private' | 'shared'
  is_owner?: boolean
  isOwner?: boolean
  owner_id?: string
  member_role?: 'owner' | 'editor' | 'viewer'
  memberRole?: 'owner' | 'editor' | 'viewer'
}

/**
 * 是否为知识库创建者
 */
export function isKBOwner(kb: KBPermissionInfo): boolean {
  return kb.is_owner === true || kb.isOwner === true
}

/**
 * 是否为直接共享知识库
 */
export function isDirectSharedKB(kb: KBPermissionInfo): boolean {
  return kb.visibility === 'shared'
}

/**
 * 获取成员角色（owner/editor/viewer）
 */
export function getKBMemberRole(kb: KBPermissionInfo): 'owner' | 'editor' | 'viewer' | null {
  const role = kb.member_role ?? kb.memberRole
  if (role && ['owner', 'editor', 'viewer'].includes(role)) {
    return role as 'owner' | 'editor' | 'viewer'
  }
  return isKBOwner(kb) ? 'owner' : null
}

/**
 * 是否可编辑知识库（创建者或编辑者）
 */
export function canEditKB(kb: KBPermissionInfo): boolean {
  if (isKBOwner(kb)) return true
  const role = getKBMemberRole(kb)
  return role === 'editor'
}

/**
 * 是否可上传/管理文档（创建者或编辑者）
 */
export function canUploadKB(kb: KBPermissionInfo): boolean {
  return canEditKB(kb)
}

/**
 * 是否可删除文档（创建者或编辑者）
 */
export function canDeleteKB(kb: KBPermissionInfo): boolean {
  return canEditKB(kb)
}

/**
 * 是否可管理知识库设置（仅创建者）
 */
export function canManageKBSettings(kb: KBPermissionInfo): boolean {
  return isKBOwner(kb)
}

/**
 * 是否可管理成员（仅创建者）
 */
export function canManageKBMembers(kb: KBPermissionInfo): boolean {
  return isKBOwner(kb)
}

/**
 * 是否可离开共享知识库（非创建者的成员）
 */
export function canLeaveKB(kb: KBPermissionInfo): boolean {
  return isDirectSharedKB(kb) && !isKBOwner(kb)
}
