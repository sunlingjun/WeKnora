<template>
  <div class="shared-kb-square-container">
    <!-- 头部 -->
    <div class="header">
      <div class="header-title">
        <h2>{{ $t('sharedKbSquare.title') }}</h2>
        <p class="header-subtitle">{{ $t('sharedKbSquare.subtitle') }}</p>
      </div>
    </div>
    <div class="header-divider"></div>

    <!-- 搜索栏 -->
    <div class="search-bar">
      <t-input
        v-model="searchKeyword"
        :placeholder="$t('sharedKbSquare.searchPlaceholder')"
        clearable
        size="large"
        @enter="handleSearch"
        @clear="handleSearch"
        class="search-input"
      >
        <template #prefix-icon>
          <t-icon name="search" />
        </template>
      </t-input>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <t-loading :loading="true" :text="$t('common.loading')" />
    </div>

    <!-- 空状态 -->
    <div v-else-if="kbs.length === 0" class="empty-container">
      <t-icon name="file-search" size="64px" />
      <p class="empty-text">{{ searchKeyword ? $t('sharedKbSquare.noSearchResult') : $t('sharedKbSquare.empty') }}</p>
    </div>

    <!-- 知识库卡片网格 -->
    <div v-else class="kb-card-grid">
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
        <!-- 卡片头部 -->
        <div class="card-header">
          <div class="card-title-wrap">
            <span class="card-title" :title="kb.name">{{ kb.name }}</span>
            <t-tag size="small" theme="success" variant="light">
              {{ $t('knowledgeList.sharedTag') }}
            </t-tag>
          </div>
        </div>

        <!-- 卡片内容 -->
        <div class="card-content">
          <p v-if="kb.description" class="card-description" :title="kb.description">
            {{ kb.description }}
          </p>
          <p v-else class="card-description empty">
            {{ $t('sharedKbSquare.noDescription') }}
          </p>
        </div>

        <!-- 卡片底部信息 -->
        <div class="card-footer">
          <div class="card-info">
            <div class="info-item">
              <t-icon name="user" size="14px" />
              <span>{{ $t('sharedKbSquare.memberCount', { count: kb.member_count || 0 }) }}</span>
            </div>
            <div class="info-item">
              <t-icon name="file" size="14px" />
              <span>{{ $t('sharedKbSquare.knowledgeCount', { count: kb.knowledge_count || 0 }) }}</span>
            </div>
          </div>
          <div class="card-actions">
            <!-- 创建者不显示离开按钮 -->
            <t-button
              v-if="kb.is_joined && !isOwner(kb)"
              theme="default"
              size="small"
              variant="outline"
              @click.stop="handleLeave(kb)"
            >
              {{ $t('knowledgeList.leave') }}
            </t-button>
            <t-button
              v-else-if="!kb.is_joined"
              theme="primary"
              size="small"
              @click.stop="handleJoin(kb)"
            >
              {{ $t('sharedKbSquare.join') }}
            </t-button>
          </div>
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
</template>

<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { listSharedKnowledgeBases, joinSharedKnowledgeBase, leaveSharedKnowledgeBase, listUserKnowledgeBases } from '@/api/knowledge-base'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
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

// 判断是否是创建者
const isOwner = (kb: KnowledgeBase) => {
  return kb.owner_id === authStore.currentUserId
}

// 搜索防抖
let searchTimer: ReturnType<typeof setTimeout> | null = null

// 获取用户已加入的知识库ID列表
const fetchUserJoinedKbs = async () => {
  try {
    const response: any = await listUserKnowledgeBases(true)
    if (response.success && response.data) {
      const userKbs = Array.isArray(response.data) ? response.data : []
      userJoinedKbIds.value = new Set(
        userKbs
          .filter((kb: any) => kb.visibility === 'shared' && kb.member_role)
          .map((kb: any) => kb.id)
      )
    }
  } catch (error) {
    console.error('Failed to fetch user joined knowledge bases:', error)
  }
}

// 加载共享知识库列表
const fetchList = async () => {
  loading.value = true
  try {
    // 先获取用户已加入的知识库
    await fetchUserJoinedKbs()
    
    const response: any = await listSharedKnowledgeBases({
      keyword: searchKeyword.value || undefined,
      page: currentPage.value,
      page_size: pageSize.value,
    })

    if (response.success) {
      // 后端返回格式: { success: true, data: [...], total: number, page: number, page_size: number }
      // data 直接是数组，total 在顶层
      let kbList: KnowledgeBase[] = []
      if (Array.isArray(response.data)) {
        kbList = response.data || []
      } else if (response.data && (response.data.items || response.data.list)) {
        kbList = response.data.items || response.data.list || []
      }
      
      // 标记已加入的知识库
      kbs.value = kbList.map((kb: KnowledgeBase) => ({
        ...kb,
        is_joined: userJoinedKbIds.value.has(kb.id),
      }))
      
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

// 搜索处理
const handleSearch = () => {
  if (searchTimer) {
    clearTimeout(searchTimer)
  }
  searchTimer = setTimeout(() => {
    currentPage.value = 1
    fetchList()
  }, 300)
}

// 分页变化
const handlePageChange = (page: number) => {
  currentPage.value = page
  fetchList()
}

// 每页数量变化
const handlePageSizeChange = (size: number) => {
  pageSize.value = size
  currentPage.value = 1
  fetchList()
}

// 加入共享知识库
const handleJoin = async (kb: KnowledgeBase) => {
  try {
    await joinSharedKnowledgeBase(kb.id)
    MessagePlugin.success(t('knowledgeList.messages.joinedSuccess'))
    // 更新本地状态
    kb.is_joined = true
    kb.member_count = (kb.member_count || 0) + 1
  } catch (error: any) {
    console.error('Failed to join shared knowledge base:', error)
    MessagePlugin.error(error.message || t('knowledgeList.messages.joinedFailed'))
  }
}

// 离开共享知识库
const handleLeave = async (kb: KnowledgeBase) => {
  try {
    await leaveSharedKnowledgeBase(kb.id)
    MessagePlugin.success(t('knowledgeList.messages.leftSuccess'))
    // 更新本地状态
    kb.is_joined = false
    kb.member_count = Math.max((kb.member_count || 0) - 1, 0)
  } catch (error: any) {
    console.error('Failed to leave shared knowledge base:', error)
    MessagePlugin.error(error.message || t('knowledgeList.messages.leftFailed'))
  }
}

// 点击卡片
const handleCardClick = (kb: KnowledgeBase) => {
  router.push(`/platform/knowledge-bases/${kb.id}`)
}

// 监听搜索关键词变化
watch(searchKeyword, () => {
  handleSearch()
})

onMounted(() => {
  fetchList()
})
</script>

<style lang="less" scoped>
.shared-kb-square-container {
  padding: 24px 44px;
  margin: 0 20px;
  height: calc(100vh);
  overflow-y: auto;
  box-sizing: border-box;
  flex: 1;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;

  .header-title {
    display: flex;
    flex-direction: column;
    gap: 4px;

    h2 {
      margin: 0;
      color: var(--td-text-color-primary);
      font-family: "PingFang SC";
      font-size: 24px;
      font-weight: 600;
      line-height: 32px;
    }

    .header-subtitle {
      margin: 0;
      color: var(--td-text-color-secondary);
      font-family: "PingFang SC";
      font-size: 14px;
      font-weight: 400;
      line-height: 20px;
    }
  }
}

.header-divider {
  height: 1px;
  background: var(--td-component-border);
  margin: 0 -44px 24px -44px;
}

.search-bar {
  margin-bottom: 24px;
  max-width: 600px;

  .search-input {
    :deep(.t-input__wrap) {
      border-radius: 8px;
      box-shadow: var(--td-shadow-1);
      transition: all 0.2s ease;

      &:hover {
        box-shadow: var(--td-shadow-2);
      }

      &:focus-within {
        box-shadow: var(--td-shadow-2);
        border-color: var(--td-brand-color);
      }
    }
  }
}

.loading-container,
.empty-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 120px 20px;
  color: var(--td-text-color-placeholder);

  .t-icon {
    color: var(--td-text-color-disabled);
    margin-bottom: 16px;
  }
}

.empty-text {
  margin-top: 16px;
  font-size: 14px;
  color: var(--td-text-color-placeholder);
  font-family: "PingFang SC";
}

.kb-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 20px;
  margin-bottom: 24px;
}

.kb-card {
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-border);
  border-radius: var(--td-radius-large);
  padding: 20px;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  flex-direction: column;
  height: 160px;
  position: relative;
  overflow: hidden;

  &:hover {
    border-color: var(--td-brand-color);
    box-shadow: var(--td-shadow-3);
    transform: translateY(-4px);
  }

  &.kb-type-document {
    //background: linear-gradient(135deg, var(--td-bg-color-container) 0%, var(--td-success-color-light) 100%);
    border-color: var(--td-success-color-2);
    
    &:hover {
      border-color: var(--td-brand-color);
      background: linear-gradient(135deg, var(--td-bg-color-container) 0%, var(--td-success-color-light) 100%);
      opacity: 1;
      box-shadow: var(--td-shadow-3);
    }

    // 右上角装饰
    &::after {
      content: '';
      position: absolute;
      top: 0;
      right: 0;
      width: 60px;
      height: 60px;
      background: linear-gradient(135deg, var(--td-brand-color-light) 0%, transparent 100%);
      border-radius: 0 var(--td-radius-large) 0 100%;
      pointer-events: none;
      z-index: 0;
    }
  }

  &.kb-type-faq {
    background: linear-gradient(135deg, var(--td-bg-color-container) 0%, var(--td-info-color-light) 100%);
    border-color: var(--td-info-color-light);
    opacity: 0.9;

    &:hover {
      border-color: var(--td-info-color);
      box-shadow: var(--td-shadow-3);
      background: linear-gradient(135deg, var(--td-bg-color-container) 0%, var(--td-info-color-light) 100%);
      opacity: 1;
    }

    // 右上角装饰
    &::after {
      content: '';
      position: absolute;
      top: 0;
      right: 0;
      width: 60px;
      height: 60px;
      background: linear-gradient(135deg, var(--td-info-color-light) 0%, transparent 100%);
      border-radius: 0 var(--td-radius-large) 0 100%;
      pointer-events: none;
      z-index: 0;
    }
  }

  // 确保内容在装饰之上
  .card-header,
  .card-content,
  .card-footer {
    position: relative;
    z-index: 1;
  }
}

.card-header {
  margin-bottom: 12px;
}

.card-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.card-title {
  color: var(--td-text-color-primary);
  font-family: "PingFang SC";
  font-size: 15px;
  font-weight: 600;
  line-height: 22px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.card-content {
  flex: 1;
  margin-bottom: 16px;
  overflow: hidden;
}

.card-description {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  overflow: hidden;
  color: var(--td-text-color-secondary);
  font-family: "PingFang SC";
  font-size: 13px;
  font-weight: 400;
  line-height: 20px;
  margin: 0;

  &.empty {
    color: var(--td-text-color-placeholder);
    font-style: italic;
  }
}

.card-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 16px;
  border-top: 1px solid var(--td-component-border);
  margin-top: auto;
}

.card-info {
  display: flex;
  gap: 20px;
  flex: 1;
}

.info-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
  font-family: "PingFang SC";

  .t-icon {
    color: var(--td-text-color-placeholder);
    flex-shrink: 0;
  }

  span {
    white-space: nowrap;
  }
}

.card-actions {
  flex-shrink: 0;
  margin-left: 12px;

  .t-button {
    min-width: 64px;
  }
}

.pagination-container {
  display: flex;
  justify-content: center;
  padding: 32px 0 24px 0;
  margin-top: 8px;
}
</style>
