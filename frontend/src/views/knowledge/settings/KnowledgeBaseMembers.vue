<template>
  <div class="kb-members-settings" :class="{ 'embedded': embedded }">
    <!-- 头部（仅非内嵌模式显示） -->
    <template v-if="!embedded">
      <div class="header">
        <div class="header-left">
          <t-button theme="default" variant="text" @click="handleBack">
            <t-icon name="chevron-left" />
            {{ $t('common.back') }}
          </t-button>
          <div class="header-title">
            <h2>{{ $t('knowledgeList.members.title') }}</h2>
            <p class="header-subtitle" v-if="kbInfo">{{ kbInfo.name }}</p>
          </div>
        </div>
      </div>
      <div class="header-divider"></div>
    </template>
    <template v-else>
      <div class="section-header embedded-header">
        <h3 class="embedded-title">{{ $t('knowledgeEditor.sidebar.members') }}</h3>
        <p class="section-description" v-if="kbInfo">{{ $t('knowledgeList.members.description', { name: kbInfo.name }) }}</p>
      </div>
    </template>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <t-loading :loading="true" :text="$t('common.loading')" />
    </div>

    <!-- 成员列表 -->
    <div v-else class="members-content">
      <!-- 搜索框 -->
      <div class="search-row">
        <t-input
          v-model="searchKeyword"
          :placeholder="$t('knowledgeList.members.searchPlaceholder')"
          clearable
          size="medium"
          class="search-input"
        >
          <template #prefix-icon>
            <t-icon name="search" />
          </template>
        </t-input>
      </div>
      <!-- 空状态 -->
      <div v-if="members.length === 0" class="empty-container">
        <t-icon name="user-circle" size="64px" />
        <p class="empty-text">{{ $t('knowledgeList.members.empty') }}</p>
      </div>

      <!-- 成员列表（卡片式，与共享设置 share-item 风格一致） -->
      <div v-else class="members-list">
        <div
          v-for="member in members"
          :key="member.id"
          class="member-item"
        >
          <div class="member-info">
            <div class="member-avatar">
              <img
                v-if="member.user?.avatar"
                :src="member.user?.avatar"
                :alt="getMemberDisplayName(member)"
              />
              <span v-else class="avatar-placeholder">{{ getMemberInitial(member) }}</span>
            </div>
            <div class="member-details">
              <div class="member-name">
                {{ member.user?.name || member.user?.real_name || member.user?.email || $t('knowledgeList.members.unknownUser') }}
              </div>
              <div v-if="member.user?.email" class="member-email">{{ member.user.email }}</div>
              <div class="member-meta">
                {{ $t('knowledgeList.members.joinedAt') }}: {{ formatDate(member.joined_at) }}
              </div>
            </div>
          </div>
          <div class="member-actions">
            <t-tag
              :theme="member.role === 'owner' ? 'success' : member.role === 'editor' ? 'primary' : 'default'"
              size="medium"
            >
              {{ getMemberRoleLabel(member.role) }}
            </t-tag>
            <t-dropdown
              v-if="isOwner && member.role !== 'owner'"
              :options="roleOptions"
              @click="(data: any) => handleMemberAction(member, data.value)"
            >
              <t-button theme="default" variant="text" size="small">
                <t-icon name="more" />
              </t-button>
            </t-dropdown>
          </div>
        </div>
      </div>

      <!-- 分页 -->
      <div v-if="total > pageSize" class="pagination-container">
        <t-pagination
          v-model="currentPage"
          :total="total"
          :page-size="pageSize"
          :show-sizer="true"
          :page-size-options="[10, 20, 50]"
          @change="handlePageChange"
          @page-size-change="handlePageSizeChange"
        />
      </div>
    </div>

    <!-- 移除成员确认对话框 -->
    <t-dialog
      v-model:visible="removeDialogVisible"
      :header="$t('knowledgeList.members.confirmRemoveTitle')"
      width="400px"
      @confirm="confirmRemoveMember"
    >
      <p>{{ $t('knowledgeList.members.confirmRemoveMessage', { name: removingMember?.user?.name || removingMember?.user?.email || $t('knowledgeList.members.unknownUser') }) }}</p>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { getKnowledgeBaseById, listKnowledgeBaseMembers, updateMemberRole, removeMember } from '@/api/knowledge-base'
import { formatStringDate } from '@/utils'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{
  kbId?: string
  embedded?: boolean
}>()

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const authStore = useAuthStore()

interface Member {
  id: string
  user_id: string
  role: 'owner' | 'editor' | 'viewer'
  joined_at: string
  user?: {
    id: string
    name?: string
    real_name?: string
    email?: string
    avatar?: string
  }
}

interface KnowledgeBase {
  id: string
  name: string
  owner_id?: string
  visibility?: 'private' | 'shared'
}

const kbId = computed(() => props.kbId || (route.params.kbId as string) || '')
const kbInfo = ref<KnowledgeBase | null>(null)
const loading = ref(false)
const members = ref<Member[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const searchKeyword = ref('')
const removeDialogVisible = ref(false)
const removingMember = ref<Member | null>(null)

const isOwner = computed(() => {
  if (!kbInfo.value || !kbInfo.value.owner_id) return false
  return kbInfo.value.owner_id === authStore.currentUserId
})

const getMemberDisplayName = (member: Member) => {
  return (
    member.user?.name ||
    member.user?.real_name ||
    member.user?.email ||
    t('knowledgeList.members.unknownUser')
  )
}

const getMemberInitial = (member: Member) => {
  const name = getMemberDisplayName(member)
  return name ? name.trim().charAt(0).toUpperCase() : '?'
}

const roleOptions = computed(() => [
  { content: t('knowledgeList.members.actions.setEditor'), value: 'editor' },
  { content: t('knowledgeList.members.actions.setViewer'), value: 'viewer' },
  { content: t('knowledgeList.members.actions.remove'), value: 'remove', divider: true }
])

const getMemberRoleLabel = (role: string) => {
  const roleMap: Record<string, string> = {
    owner: t('knowledgeList.role.owner'),
    editor: t('knowledgeList.role.editor'),
    viewer: t('knowledgeList.role.viewer')
  }
  return roleMap[role] || role
}

const formatDate = (date: string) => {
  if (!date) return '-'
  return formatStringDate(date)
}

const fetchKBInfo = async () => {
  if (!kbId.value) return
  try {
    const response: any = await getKnowledgeBaseById(kbId.value)
    if (response.success && response.data) {
      kbInfo.value = response.data
    }
  } catch (error: any) {
    console.error('Failed to fetch knowledge base info:', error)
    MessagePlugin.error(error.message || t('common.loadFailed'))
  }
}

const fetchMembers = async () => {
  if (!kbId.value) return
  loading.value = true
  try {
    const response: any = await listKnowledgeBaseMembers(kbId.value, {
      keyword: searchKeyword.value.trim() || undefined,
      page: currentPage.value,
      page_size: pageSize.value
    })
    if (response.success) {
      if (Array.isArray(response.data)) {
        members.value = response.data
      } else if (response.data?.items) {
        members.value = response.data.items
      } else if (response.data?.list) {
        members.value = response.data.list
      } else {
        members.value = []
      }
      total.value = response.total || 0
    } else {
      members.value = []
      total.value = 0
    }
  } catch (error: any) {
    console.error('Failed to fetch members:', error)
    MessagePlugin.error(error.message || t('knowledgeList.messages.fetchMembersFailed'))
    members.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

const handleMemberAction = (member: Member, action: string) => {
  if (action === 'remove') {
    removingMember.value = member
    removeDialogVisible.value = true
  } else if (action === 'editor' || action === 'viewer') {
    handleUpdateMemberRole(member.user_id, action)
  }
}

const handleUpdateMemberRole = async (userId: string, role: 'editor' | 'viewer') => {
  try {
    await updateMemberRole(kbId.value, userId, role)
    MessagePlugin.success(t('knowledgeList.messages.roleUpdated'))
    fetchMembers()
  } catch (error: any) {
    console.error('Failed to update member role:', error)
    MessagePlugin.error(error.message || t('knowledgeList.messages.roleUpdateFailed'))
  }
}

const confirmRemoveMember = async () => {
  if (!removingMember.value) return
  try {
    await removeMember(kbId.value, removingMember.value.user_id)
    MessagePlugin.success(t('knowledgeList.messages.memberRemoved'))
    removeDialogVisible.value = false
    removingMember.value = null
    fetchMembers()
  } catch (error: any) {
    console.error('Failed to remove member:', error)
    MessagePlugin.error(error.message || t('knowledgeList.messages.memberRemoveFailed'))
  }
}

const handlePageChange = (page: number) => {
  currentPage.value = page
  fetchMembers()
}

const handlePageSizeChange = (size: number) => {
  pageSize.value = size
  currentPage.value = 1
  fetchMembers()
}

const handleBack = () => {
  router.back()
}

// debounce 搜索：输入停止 300ms 后触发
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null
watch(searchKeyword, () => {
  currentPage.value = 1
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
  searchDebounceTimer = setTimeout(() => {
    fetchMembers()
    searchDebounceTimer = null
  }, 300)
})

onMounted(() => {
  fetchKBInfo()
  fetchMembers()
})
</script>

<style lang="less" scoped>
.kb-members-settings {
  width: 100%;
}

/* 独立页面头部 */
.header {
  margin-bottom: 16px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-title {
  h2 {
    font-size: 20px;
    font-weight: 600;
    color: var(--td-text-color-primary);
    margin: 0 0 4px 0;
  }

  .header-subtitle {
    font-size: 14px;
    color: var(--td-text-color-secondary);
    margin: 0;
  }
}

.header-divider {
  height: 1px;
  background: var(--td-component-border);
  margin-bottom: 24px;
}

/* 内嵌模式头部（与高级设置 section-header 一致） */
.section-header.embedded-header {
  margin-bottom: 32px;
}

.embedded-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin: 0 0 8px 0;
}

.section-description {
  font-size: 14px;
  color: var(--td-text-color-secondary);
  margin: 0;
  line-height: 1.5;
}

/* 加载与空状态 */
.loading-container,
.empty-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 20px;
  color: var(--td-text-color-placeholder);
}

.empty-text {
  margin-top: 16px;
  font-size: 14px;
  color: var(--td-text-color-placeholder);
}

/* 搜索框 */
.search-row {
  margin-bottom: 16px;
}

.search-input {
  max-width: 280px;
}

/* 成员列表内容区 */
.members-content {
  min-height: 200px;
}

.members-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 0;
}

/* 成员卡片：与 KBShareSettings share-item 风格一致 */
.member-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  background: var(--td-bg-color-secondarycontainer, #fafafa);
  border: 1px solid var(--td-component-border, #f0f0f0);
  border-radius: 8px;
  transition: background 0.2s ease, border-color 0.2s ease;

  &:hover {
    background: var(--td-bg-color-secondarycontainer-hover, #f5f5f5);
    border-color: var(--td-component-stroke, #e8e8e8);
  }
}

.member-info {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 16px;
}

.member-avatar {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  overflow: hidden;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .avatar-placeholder {
    width: 100%;
    height: 100%;
    background: linear-gradient(135deg, rgba(from var(--td-brand-color) r g b / 0.12), rgba(from var(--td-brand-color) r g b / 0.24));
    color: var(--td-brand-color);
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 600;
    font-size: 16px;
    text-transform: uppercase;
  }
}

.member-details {
  flex: 1;
  min-width: 0;
}

.member-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--td-text-color-primary);
  margin-bottom: 2px;
}

.member-email {
  font-size: 13px;
  color: var(--td-text-color-secondary);
  margin-bottom: 2px;
}

.member-meta {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.member-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 12px;
}

.pagination-container {
  display: flex;
  justify-content: center;
  padding: 24px 0;
  margin-top: 20px;
}
</style>
