<script setup lang="ts">
/* Nothing private renders until the session has an authorized metadata
   response; every other state is a notice. */
import { onBeforeUnmount, onMounted } from 'vue'
import BinPage from './components/BinPage.vue'
import SessionNotice from './components/SessionNotice.vue'
import { browserEnvironment, createSession } from './lib/session'
import { useThemeAttribute } from './lib/theme'

useThemeAttribute()

const session = createSession(browserEnvironment())
const { page, state, access, message } = session

onMounted(() => void session.start())
onBeforeUnmount(() => session.stop())
</script>

<template>
  <BinPage
    v-if="state === 'ready' && access"
    :access="access"
    :origin="page.origin"
    :selected-request-id="page.requestId"
  />
  <SessionNotice v-else :state="state" :message="message" @retry="session.retry()" />
</template>
