<template>
  <div class="shared-kb-square-container">
    <div class="square-content">
      <div class="header">
        <div class="header-title">
          <h2>{{ $t('sharedKbSquare.title') }}</h2>
          <p class="header-subtitle">{{ $t('sharedKbSquare.subtitle') }}</p>
        </div>
      </div>

      <div class="square-main">
        <div class="search-bar">
          <t-input
            v-model="searchKeyword"
            :placeholder="$t('sharedKbSquare.searchPlaceholder')"
            clearable
            @enter="handleSearch"
            @clear="handleSearch"
            class="search-input"
          >
            <template #prefix-icon>
              <t-icon name="search" />
            </template>
          </t-input>
        </div>

        <!-- ??? -->
        <div v-if="loading && kbs.length === 0" class="kb-card-wrap">
          <div v-for="n in 6" :key="'skel-' + n" class="kb-card kb-card-skeleton">
            <div class="card-header">
              <t-skeleton animation="gradient" :row-col="[{ width: '60%', height: '20px' }]" />
            </div>
            <div class="card-content">
              <t-skeleton
                animation="gradient"
                :row-col="[{ width: '100%', height: '14px' }, { width: '80%', height: '14px' }]"
              />
            </div>
            <div class="card-bottom">
              <t-skeleton
                animation="gradient"
                :row-col="[[{ width: '28px', height: '28px', type: 'rect' }, { width: '28px', height: '28px', type: 'rect' }]]"
              />
            </div>
          </div>
        </div>

        <!-- ??? -->
        <div v-else-if="!loading && kbs.length === 0" class="empty-state">
          <t-icon name="file-search" size="48px" class="empty-icon" />
          <p class="empty-txt">
            {{ searchKeyword ? $t('sharedKbSquare.noSearchResult') : $t('sharedKbSquare.empty') }}
          </p>
        </div>

        <!-- ??????? -->
        <div v-else class="kb-card-wrap">
          <div
            v-for="kb in kbs"
            :key="kb.id"
            class="kb-card"
            :class="{
              'kb-type-document': (kb.type || 'document') === 'document',
              'kb-type-faq': kb.type === 'faq',
            }"
            @click="handleCardClick(kb)"
          >
            <div class="card-header">
              <div class="card-title-wrap">
                <span class="card-title" :title="kb.name">{{ kb.name }}</span>
                <t-tag size="small" theme="success" variant="light">
                  {{ $t('knowledgeList.sharedTag') }}
                </t-tag>
                <t-tag v-if="isOwner(kb)" size="small" theme="primary" variant="light">
                  {{ $t('knowledgeList.role.owner') }}
                </t-tag>
              </div>
            </div>

            <div class="card-content">
              <div class="card-description">
                {{ kb.description || $t('sharedKbSquare.noDescription') }}
              </div>
            </div>

            <div class="card-bottom">
              <div class="bottom-left">
                <div class="feature-badges">
                  <t-tooltip
                    :content="kb.type === 'faq' ? $t('knowledgeEditor.basic.typeFAQ') : $t('knowledgeEditor.basic.typeDocument')"
                    placement="top"
                  >
                    <div
                      class="feature-badge"
                      :class="{
                        'type-document': (kb.type || 'document') === 'document',
                        'type-faq': kb.type === 'faq',
                      }"
                    >
                      <t-icon :name="kb.type === 'faq' ? 'chat-bubble-help' : 'folder'" size="14px" />
                      <span class="badge-count">{{ kb.knowledge_count || 0 }}</span>
                    </div>
                  </t-tooltip>
                  <t-tooltip
                    :content="$t('sharedKbSquare.memberCount', { count: kb.member_count || 0 })"
                    placement="top"
                  >
                    <div class="feature-badge members">
                      <t-icon name="user" size="14px" />
                      <span class="badge-count">{{ kb.member_count || 0 }}</span>
                    </div>
                  </t-tooltip>
                </div>
              </div>
              <div class="bottom-right" @click.stop>
                <t-button
                  v-if="kb.is_joined && !isOwner(kb)"
                  theme="default"
                  size="small"
                  variant="outline"
                  @click="handleLeave(kb)"
                >
                  {{ $t('knowledgeList.leave') }}
                </t-button>
                <t-button
                  v-else-if="!kb.is_joined"
                  theme="primary"
                  size="small"
                  variant="outline"
                  @click="handleJoin(kb)"
                >
                  {{ $t('sharedKbSquare.join') }}
                </t-button>
              </div>
            </div>
          </div>
        </div>

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
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  listSharedKnowledgeBases,
  joinSharedKnowledgeBase,
  leaveSharedKnowledgeBase,
  listUserKnowledgeBases,
} from '@/api/knowledge-base'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const authStore = useAuthStore()

interface KnowledgeBase {
  id: string
  name: string
  description?: string
  type?: 'document' | 'faq'
  visibility: 'private' | 'shared'
  member_count?: number
  knowledge_count?: number
  is_joined?: boolean
  owner_id?: string
  shared_at?: string
}

const searchKeyword = ref('')
const loading = ref(false)
const kbs = ref<KnowledgeBase[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const userJoinedKbIds = ref<Set<string>>(new Set())
const joinedLoaded = ref(false)

const isOwner = (kb: KnowledgeBase) => {
  const userId = authStore.user?.id
  return !!(userId && kb.owner_id && kb.owner_id === userId)
}

const fetchUserJoinedKbs = async (force = false) => {
  // ?????????????/??????????????? /user?
  if (!force && joinedLoaded.value) return
  try {
    const res: any = await listUserKnowledgeBases(true)
    const list = Array.isArray(res?.data) ? res.data : []
    userJoinedKbIds.value = new Set(
      list
        .filter((kb: any) => kb.visibility === 'shared')
        .map((kb: any) => kb.id as string),
    )
    joinedLoaded.value = true
  } catch (error) {
    console.error('Failed to fetch user joined knowledge bases:', error)
  }
}

const applyJoinedFlags = (list: KnowledgeBase[]) =>
  list.map((kb) => ({
    ...kb,
    is_joined: userJoinedKbIds.value.has(kb.id),
  }))

const fetchList = async (opts?: { refreshJoined?: boolean }) => {
  loading.value = true
  try {
    await fetchUserJoinedKbs(!!opts?.refreshJoined)

    const response: any = await listSharedKnowledgeBases({
      keyword: searchKeyword.value || undefined,
      page: currentPage.value,
      page_size: pageSize.value,
    })

    if (response.success) {
      let kbList: KnowledgeBase[] = []
      if (Array.isArray(response.data)) {
        kbList = response.data || []
      } else if (response.data && (response.data.items || response.data.list)) {
        kbList = response.data.items || response.data.list || []
      }

      kbs.value = applyJoinedFlags(kbList)
      total.value = response.total || 0
    } else {
      kbs.value = []
      total.value = 0
    }
  } catch (error: any) {
    console.error('Failed to fetch shared knowledge bases:', error)
    MessagePlugin.error(error.message || t('sharedKbSquare.fetchFailed'))
    kbs.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  currentPage.value = 1
  fetchList()
}

const handlePageChange = (pageInfo: { current: number }) => {
  currentPage.value = pageInfo.current
  fetchList()
}

const handlePageSizeChange = (size: number) => {
  pageSize.value = size
  currentPage.value = 1
  fetchList()
}

const handleCardClick = (kb: KnowledgeBase) => {
  if (kb.is_joined || isOwner(kb)) {
    router.push(`/platform/knowledge-bases/${kb.id}`)
  }
}

const handleJoin = async (kb: KnowledgeBase) => {
  try {
    await joinSharedKnowledgeBase(kb.id)
    MessagePlugin.success(t('knowledgeList.messages.joinedSuccess'))
    userJoinedKbIds.value.add(kb.id)
    kb.is_joined = true
    kb.member_count = (kb.member_count || 0) + 1
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('knowledgeList.messages.joinedFailed'))
  }
}

const handleLeave = async (kb: KnowledgeBase) => {
  try {
    await leaveSharedKnowledgeBase(kb.id)
    MessagePlugin.success(t('knowledgeList.messages.leftSuccess'))
    userJoinedKbIds.value.delete(kb.id)
    kb.is_joined = false
    kb.member_count = Math.max((kb.member_count || 0) - 1, 0)
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('knowledgeList.messages.leftFailed'))
  }
}

let searchTimer: ReturnType<typeof setTimeout> | null = null
watch(searchKeyword, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => handleSearch(), 300)
})

const handleSquareRefresh = () => {
  fetchList({ refreshJoined: true })
}

watch(
  () => route.name,
  (name) => {
    if (name === 'sharedKnowledgeBaseSquare') handleSquareRefresh()
  },
  { immediate: true },
)

onMounted(() => {
  window.addEventListener('weknora:refresh-shared-kb-square', handleSquareRefresh as EventListener)
})

onUnmounted(() => {
  window.removeEventListener('weknora:refresh-shared-kb-square', handleSquareRefresh as EventListener)
})
</script>

<style lang="less" scoped>
.shared-kb-square-container {
  margin: 0;
  height: 100%;
  box-sizing: border-box;
  flex: 1;
  display: flex;
  position: relative;
  min-height: 0;
}

.square-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 20px 0 0 28px;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  padding-right: 28px;

  .header-title {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  h2 {
    margin: 0;
    color: var(--td-text-color-primary);
    font-family: var(--app-font-family);
    font-size: 24px;
    font-weight: 600;
    line-height: 32px;
  }
}

.header-subtitle {
  margin: 0;
  color: var(--td-text-color-placeholder);
  font-family: var(--app-font-family);
  font-size: 14px;
  font-weight: 400;
  line-height: 20px;
}

.square-main {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 0 28px 8px 0;
  display: flex;
  flex-direction: column;
}

.search-bar {
  margin-bottom: 16px;
  max-width: 360px;

  .search-input {
    :deep(.t-input) {
      border-radius: 8px;
    }
  }
}

@keyframes contentFadeIn {
  from {
    opacity: 0;
    transform: translateY(6px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.kb-card-wrap {
  display: grid;
  gap: 12px;
  grid-template-columns: 1fr;
  animation: contentFadeIn 0.32s ease-out;
}

.kb-card {
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  overflow: hidden;
  box-sizing: border-box;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
  background: var(--td-bg-color-container);
  position: relative;
  cursor: pointer;
  transition: all 0.25s ease;
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  height: 136px;
  min-height: 136px;

  &.kb-card-skeleton {
    cursor: default;

    .card-header {
      margin-bottom: 12px;
    }

    .card-content {
      flex: 1;
    }

    .card-bottom {
      margin-top: auto;
    }
  }

  &:hover {
    border-color: var(--td-brand-color);
    box-shadow: 0 4px 12px color-mix(in srgb, var(--td-brand-color) 12%, transparent);
  }

  &.kb-type-document {
    background: linear-gradient(135deg, var(--td-bg-color-container) 0%, color-mix(in srgb, var(--td-brand-color) 4%, transparent) 100%);

    &:hover {
      border-color: var(--td-brand-color);
      background: linear-gradient(135deg, var(--td-bg-color-container) 0%, color-mix(in srgb, var(--td-brand-color) 8%, transparent) 100%);
    }

    &::after {
      content: '';
      position: absolute;
      top: 0;
      right: 0;
      width: 60px;
      height: 60px;
      background: linear-gradient(135deg, color-mix(in srgb, var(--td-brand-color) 8%, transparent) 0%, transparent 100%);
      border-radius: 0 12px 0 100%;
      pointer-events: none;
      z-index: 0;
    }
  }

  &.kb-type-faq {
    background: linear-gradient(135deg, var(--td-bg-color-container) 0%, rgba(0, 82, 217, 0.04) 100%);

    &:hover {
      border-color: var(--td-brand-color);
      box-shadow: 0 4px 12px rgba(0, 82, 217, 0.12);
      background: linear-gradient(135deg, var(--td-bg-color-container) 0%, rgba(0, 82, 217, 0.08) 100%);
    }

    &::after {
      content: '';
      position: absolute;
      top: 0;
      right: 0;
      width: 60px;
      height: 60px;
      background: linear-gradient(135deg, rgba(0, 82, 217, 0.08) 0%, transparent 100%);
      border-radius: 0 12px 0 100%;
      pointer-events: none;
      z-index: 0;
    }
  }

  .card-header,
  .card-content,
  .card-bottom {
    position: relative;
    z-index: 1;
  }
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 4px;
  margin-bottom: 6px;
}

.card-title-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
}

.card-title {
  color: var(--td-text-color-primary);
  font-family: var(--app-font-family);
  font-size: 15px;
  font-weight: 600;
  line-height: 22px;
  letter-spacing: 0.01em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.card-content {
  flex: 1;
  min-height: 0;
  margin-bottom: 6px;
  overflow: hidden;
}

.card-description {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  overflow: hidden;
  color: var(--td-text-color-secondary);
  font-family: var(--app-font-family);
  font-size: 12px;
  font-weight: 400;
  line-height: 17px;
}

.card-bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: auto;
  padding-top: 6px;
  border-top: 0.5px solid var(--td-component-stroke);
}

.bottom-left {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.bottom-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.feature-badges {
  display: flex;
  align-items: center;
  gap: 4px;
}

.feature-badge {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 22px;
  border-radius: 5px;
  cursor: default;
  transition: background 0.2s ease;
  width: auto;
  padding: 0 6px;
  gap: 3px;

  .badge-count {
    font-size: 11px;
    font-weight: 500;
  }

  &.type-document {
    background: color-mix(in srgb, var(--td-brand-color) 8%, transparent);
    color: var(--td-brand-color-active);

    &:hover {
      background: color-mix(in srgb, var(--td-brand-color) 12%, transparent);
    }
  }

  &.type-faq {
    background: rgba(0, 82, 217, 0.08);
    color: var(--td-brand-color);

    &:hover {
      background: rgba(0, 82, 217, 0.12);
    }
  }

  &.members {
    background: var(--td-bg-color-secondarycontainer);
    color: var(--td-text-color-secondary);

    &:hover {
      background: var(--td-bg-color-component);
    }
  }
}

.empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  padding: 60px 20px;

  .empty-icon {
    color: var(--td-text-color-disabled);
    margin-bottom: 16px;
  }

  .empty-txt {
    color: var(--td-text-color-placeholder);
    font-family: var(--app-font-family);
    font-size: 16px;
    font-weight: 600;
    line-height: 26px;
    margin: 0;
  }
}

.pagination-container {
  display: flex;
  justify-content: center;
  padding: 20px 0 12px;
}

@media (min-width: 900px) {
  .kb-card-wrap {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (min-width: 1250px) {
  .kb-card-wrap {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (min-width: 1600px) {
  .kb-card-wrap {
    grid-template-columns: repeat(4, 1fr);
  }
}

@media (min-width: 1900px) {
  .kb-card-wrap {
    grid-template-columns: repeat(5, 1fr);
  }
}

@media (min-width: 2200px) {
  .kb-card-wrap {
    grid-template-columns: repeat(6, 1fr);
  }
}
</style>
