<template>
  <main class="join-cas">
    <section class="join-cas-card">
      <img
        class="join-cas-brand"
        src="@/assets/img/nxin-weknora.svg"
        alt="NXIN-ZSK"
        width="160"
        height="40"
      />
      <h1>{{ $t('joinCas.title') }}</h1>

      <template v-if="invite">
        <p class="join-cas-tenant">{{ invite.tenant_name || invite.tenant_id }}</p>
        <p v-if="roleText" class="join-cas-role">
          {{ $t('joinCas.roleLabel', { role: roleText }) }}
        </p>
      </template>

      <div v-if="phase === 'loading'" class="join-cas-status">
        <t-loading size="small" />
        <span>
          {{
            invite
              ? $t('joinCas.loading', { tenant: invite.tenant_name || invite.tenant_id })
              : $t('inviteRegister.loading')
          }}
        </span>
      </div>

      <div v-else-if="phase === 'error'" class="join-cas-error" role="alert">
        <t-icon name="error-circle" size="20px" aria-hidden="true" />
        <span>{{ errorMessage }}</span>
        <t-button theme="primary" size="small" @click="runFlow">
          {{ $t('joinCas.retry') }}
        </t-button>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  getInvitationByToken,
  joinByInviteCas,
  userInfoFromApi,
  type InviteLookup,
} from '@/api/auth'
import { useRoleLabel } from '@/composables/useRoleLabel'
import { useAuthStore } from '@/stores/auth'
import { useCASStore } from '@/stores/cas'
import { navigateAfterTenantSwitch } from '@/utils/tenantSwitch'
import { needsCasIdentityReconcile } from '@/utils/casSession'

const route = useRoute()
const { t } = useI18n()
const { formatRole } = useRoleLabel()
const authStore = useAuthStore()
const casStore = useCASStore()

const phase = ref<'loading' | 'error'>('loading')
const errorMessage = ref('')
const invite = ref<InviteLookup | null>(null)
let running = false

const inviteToken = computed(() => {
  const raw = route.query.token
  const value = Array.isArray(raw) ? raw[0] : raw
  return typeof value === 'string' ? value.trim() : ''
})

const roleText = computed(() => {
  if (!invite.value?.role) return ''
  return formatRole(invite.value.role) || invite.value.role
})

function applyLoginResponse(response: {
  user?: any
  token?: string
  refresh_token?: string
  tenant?: any
  active_tenant?: any
  memberships?: any[]
}) {
  // Mirror Login.vue persistLoginResponse: home tenant on user, active
  // tenant via setTenant / setSelectedTenant.
  const activeTenant = response.active_tenant || response.tenant
  if (!response.user || !response.token) return false

  const homeTenantIdRaw = response.user.tenant_id ?? activeTenant?.id ?? ''
  authStore.setUser(userInfoFromApi(response.user, homeTenantIdRaw))
  authStore.setToken(response.token)
  if (response.refresh_token) {
    authStore.setRefreshToken(response.refresh_token)
  }
  if (activeTenant) {
    authStore.setTenant({
      id: String(activeTenant.id) || '',
      name: activeTenant.name || '',
      owner_id: response.user.id || '',
      created_at: activeTenant.created_at || new Date().toISOString(),
      updated_at: activeTenant.updated_at || new Date().toISOString(),
    })
  } else {
    authStore.setTenant(null)
  }
  if (Array.isArray(response.memberships)) {
    authStore.setMemberships(response.memberships)
  }
  const activeIdNum = Number(activeTenant?.id)
  const homeIdNum = Number(homeTenantIdRaw)
  if (Number.isFinite(activeIdNum) && Number.isFinite(homeIdNum) && activeIdNum !== homeIdNum) {
    authStore.setSelectedTenant(activeIdNum, activeTenant?.name || null)
  } else {
    authStore.setSelectedTenant(null, null)
  }
  return true
}

async function runFlow() {
  if (running) return
  running = true
  phase.value = 'loading'
  errorMessage.value = ''
  invite.value = null

  try {
    const token = inviteToken.value
    if (!token) {
      phase.value = 'error'
      errorMessage.value = t('joinCas.invalidToken')
      return
    }

    const lookup = await getInvitationByToken(token)
    if (!lookup.success || !lookup.data) {
      phase.value = 'error'
      errorMessage.value = lookup.message || t('joinCas.invalidToken')
      return
    }
    invite.value = lookup.data

    // NXIN: CAS cookies are HttpOnly, so this page cannot compare them to a
    // stored JWT. Re-validate once per full page load; after that, use the
    // current session. Otherwise a leftover JWT could join as the previous user.
    if (needsCasIdentityReconcile() || !authStore.isLoggedIn) {
      const casOk = await casStore.validateSession()
      if (!casOk) {
        // Redirect already in flight; keep loading UI until unload.
        return
      }
    }

    const joinPayload: { token: string; refresh_token?: string } = { token }
    if (authStore.refreshToken) {
      joinPayload.refresh_token = authStore.refreshToken
    }
    const response = await joinByInviteCas(joinPayload)
    if (!response.success || !response.token) {
      // Join failed after CAS validate: interceptor may have cleared
      // localStorage while Pinia still looks logged-in, so retry would skip
      // CAS. Drop the session so a retry can validate again.
      if (authStore.isLoggedIn) {
        authStore.logout()
      }
      phase.value = 'error'
      errorMessage.value = response.message || t('joinCas.joinFailed')
      return
    }

    if (!applyLoginResponse(response)) {
      phase.value = 'error'
      errorMessage.value = t('joinCas.joinFailed')
      return
    }

    navigateAfterTenantSwitch()
  } catch (err: any) {
    phase.value = 'error'
    errorMessage.value = err?.message || t('joinCas.joinFailed')
  } finally {
    running = false
  }
}

onMounted(() => {
  document.title = t('joinCas.documentTitle')
  void runFlow()
})

watch(inviteToken, (next, prev) => {
  if (next && next !== prev) {
    void runFlow()
  }
})
</script>

<style scoped lang="less">
.join-cas {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 32px 20px;
  background:
    radial-gradient(circle at 20% 10%, color-mix(in srgb, var(--td-brand-color) 12%, transparent), transparent 38%),
    var(--td-bg-color-page);
}

.join-cas-card {
  width: min(480px, 100%);
  padding: 44px 40px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 20px;
  background: var(--td-bg-color-container);
  box-shadow: var(--td-shadow-2);
  text-align: center;
}

.join-cas-brand {
  display: block;
  margin: 0 auto 20px;
  max-width: 180px;
  height: auto;
}

h1 {
  margin: 0 0 12px;
  color: var(--td-text-color-primary);
  font-size: 24px;
  line-height: 1.3;
}

.join-cas-tenant {
  margin: 0 0 6px;
  color: var(--td-text-color-primary);
  font-size: 16px;
  font-weight: 600;
}

.join-cas-role {
  margin: 0 0 20px;
  color: var(--td-text-color-secondary);
  font-size: 14px;
}

.join-cas-status,
.join-cas-error {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 52px;
  color: var(--td-text-color-secondary);
  font-size: 14px;
}

.join-cas-error {
  flex-wrap: wrap;
  padding: 12px 16px;
  border-radius: 10px;
  color: var(--td-error-color);
  background: var(--td-error-color-light);
}

@media (max-width: 560px) {
  .join-cas-card {
    padding: 32px 22px;
  }
}
</style>
