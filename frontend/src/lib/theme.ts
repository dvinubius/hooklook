/** Theme preference. Dark is the default; the shell already ships dark so the
 *  page never flashes light before this runs. */

import { ref, watchEffect } from 'vue'

export type Theme = 'dark' | 'light'

const storageKey = 'hooklook.theme'

function stored(): Theme {
  try {
    return localStorage.getItem(storageKey) === 'light' ? 'light' : 'dark'
  } catch {
    return 'dark'
  }
}

export const theme = ref<Theme>(stored())

export function useThemeAttribute(): void {
  watchEffect(() => {
    document.documentElement.setAttribute('data-theme', theme.value)
    try {
      localStorage.setItem(storageKey, theme.value)
    } catch {
      // A blocked storage API is not a reason to lose the working toggle.
    }
  })
}

export function toggleTheme(): void {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
}
