<script setup lang="ts">
/* Nothing private renders until the session has an authorized metadata
   response; every other state is a notice. */
import { onBeforeUnmount, onMounted } from 'vue'
import BinUnavailable from './components/BinUnavailable.vue'
import BinPage from './components/BinPage.vue'
import SessionNotice from './components/SessionNotice.vue'
import { browserEnvironment, createSession } from './lib/session'
import { useThemeAttribute } from './lib/theme'

useThemeAttribute()

const session = createSession(browserEnvironment())
const { state, access, message } = session

onMounted(() => void session.start())
onBeforeUnmount(() => session.stop())
</script>

<template>
  <BinPage v-if="state === 'ready' && access" :session="session" :access="access" />
  <BinUnavailable v-else-if="state === 'bin_expired'" kind="expired" />
  <BinUnavailable v-else-if="state === 'shared_bin_unavailable'" kind="shared" />
  <SessionNotice v-else :state="state" :message="message" @retry="session.retry()" />
</template>
