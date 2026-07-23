<template>
  <div class="relative min-h-screen overflow-x-hidden bg-slate-50 text-slate-950 dark:bg-dark-950 dark:text-white">
    <div class="pointer-events-none absolute inset-0">
      <div class="absolute inset-0 bg-[linear-gradient(rgba(15,23,42,0.045)_1px,transparent_1px),linear-gradient(90deg,rgba(15,23,42,0.045)_1px,transparent_1px)] bg-[size:56px_56px] dark:bg-[linear-gradient(rgba(148,163,184,0.08)_1px,transparent_1px),linear-gradient(90deg,rgba(148,163,184,0.08)_1px,transparent_1px)]"></div>
      <div class="absolute inset-x-0 top-0 h-40 bg-gradient-to-b from-white to-transparent dark:from-dark-950"></div>
      <div class="absolute bottom-0 right-0 h-72 w-1/2 bg-gradient-to-tl from-primary-100/70 via-white/0 to-transparent dark:from-primary-950/25"></div>
    </div>

    <div class="relative mx-auto flex min-h-screen w-full max-w-7xl items-center px-4 py-8 sm:px-6 lg:px-8">
      <div class="grid w-full gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(380px,460px)] lg:items-stretch">
        <section
          class="hidden min-h-[620px] overflow-hidden rounded-lg border border-slate-200/80 bg-white/80 shadow-card backdrop-blur-xl dark:border-dark-700/70 dark:bg-dark-900/70 lg:flex"
        >
          <div class="relative flex w-full flex-col justify-between p-10 xl:p-12">
            <div class="absolute inset-0 bg-[linear-gradient(135deg,rgba(20,184,166,0.10),transparent_35%),linear-gradient(315deg,rgba(245,158,11,0.10),transparent_32%)]"></div>

            <div class="relative">
              <div class="flex items-center gap-4">
                <div class="flex h-14 w-14 items-center justify-center overflow-hidden rounded-lg bg-white shadow-sm ring-1 ring-slate-200 dark:bg-dark-800 dark:ring-dark-700">
                  <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
                </div>
                <div class="min-w-0">
                  <p class="truncate text-xl font-bold tracking-normal text-slate-950 dark:text-white">
                    {{ siteName }}
                  </p>
                  <p class="mt-1 text-sm text-slate-500 dark:text-dark-300">
                    {{ siteSubtitle }}
                  </p>
                </div>
              </div>

              <div class="mt-16 max-w-xl">
                <p class="text-sm font-semibold text-primary-700 dark:text-primary-300">
                  {{ t('home.heroKicker') }}
                </p>
                <h1 class="mt-4 text-4xl font-bold leading-tight tracking-normal text-slate-950 dark:text-white xl:text-5xl">
                  {{ t('home.heroSubtitle') }}
                </h1>
                <p class="mt-5 max-w-lg text-base leading-7 text-slate-600 dark:text-dark-200">
                  {{ t('home.heroDescription') }}
                </p>
              </div>
            </div>

            <div class="relative grid gap-3">
              <div
                v-for="item in capabilityItems"
                :key="item.title"
                class="flex items-center gap-3 px-1 py-2"
              >
                <span class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-slate-100 text-primary-700 dark:bg-dark-700 dark:text-primary-300">
                  <Icon :name="item.icon" size="sm" />
                </span>
                <span class="text-sm font-semibold text-slate-800 dark:text-dark-100">
                  {{ item.title }}
                </span>
              </div>
            </div>
          </div>
        </section>

        <main class="flex w-full flex-col justify-center">
          <div class="mb-6 flex items-center gap-3 lg:hidden">
            <div class="flex h-12 w-12 flex-shrink-0 items-center justify-center overflow-hidden rounded-lg bg-white shadow-sm ring-1 ring-slate-200 dark:bg-dark-800 dark:ring-dark-700">
              <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
            </div>
            <div class="min-w-0">
              <p class="truncate text-lg font-bold text-slate-950 dark:text-white">{{ siteName }}</p>
              <p class="truncate text-sm text-slate-500 dark:text-dark-300">{{ siteSubtitle }}</p>
            </div>
          </div>

          <div class="rounded-lg border border-slate-200/90 bg-white/95 p-6 shadow-card-hover backdrop-blur-xl dark:border-dark-700/80 dark:bg-dark-900/90 sm:p-8">
            <slot />
          </div>

          <div class="mt-6 text-center text-sm">
            <slot name="footer" />
          </div>

          <div class="mt-8 text-center text-xs text-slate-400 dark:text-dark-500">
            &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
          </div>
        </main>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import { displaySiteName } from '@/utils/siteName'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()
const { t, locale } = useI18n()

const siteName = computed(() => displaySiteName(appStore.siteName, locale.value))
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => {
  const subtitle = appStore.cachedPublicSettings?.site_subtitle?.trim()
  if (!subtitle || subtitle === 'Subscription to API Conversion Platform') {
    return 'AI Infrastructure Platform'
  }
  return subtitle
})

const currentYear = computed(() => new Date().getFullYear())
type CapabilityItem = {
  icon: 'server' | 'shield' | 'chartBar'
  title: string
}

const capabilityItems = computed<CapabilityItem[]>(() => [
  { icon: 'server', title: t('home.tags.subscriptionToApi') },
  { icon: 'shield', title: t('home.tags.stickySession') },
  { icon: 'chartBar', title: t('home.tags.realtimeBilling') },
])

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>
