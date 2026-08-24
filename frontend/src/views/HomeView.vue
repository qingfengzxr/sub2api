<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- Default Home Page -->
  <div v-else class="min-h-screen bg-slate-50 text-slate-950 transition-colors dark:bg-slate-950 dark:text-white">
    <section class="relative min-h-[88vh] overflow-hidden bg-slate-100 dark:bg-slate-950">
      <picture class="absolute inset-0 block h-full w-full">
        <source
          type="image/webp"
          srcset="/frontpage/2-960.webp 960w, /frontpage/2-1440.webp 1440w, /frontpage/2-1920.webp 1920w, /frontpage/2-2560.webp 2560w"
          sizes="100vw"
        />
        <img
          src="/frontpage/2.png"
          alt=""
          class="h-full w-full object-cover object-[58%_center] sm:object-center"
          width="2560"
          height="1440"
          fetchpriority="high"
          decoding="async"
        />
      </picture>
      <div class="absolute inset-0 bg-[linear-gradient(90deg,rgba(248,250,252,0.94)_0%,rgba(241,245,249,0.82)_38%,rgba(241,245,249,0.28)_72%,rgba(248,250,252,0.58)_100%)] dark:bg-[linear-gradient(90deg,rgba(2,6,23,0.88)_0%,rgba(7,20,37,0.72)_38%,rgba(7,20,37,0.24)_72%,rgba(2,6,23,0.5)_100%)]"></div>
      <div class="absolute inset-x-0 bottom-0 h-40 bg-gradient-to-t from-slate-50 to-transparent dark:from-slate-950"></div>

      <header class="relative z-20 px-5 py-4 sm:px-6 lg:px-8">
        <nav class="mx-auto flex max-w-7xl items-center justify-between gap-4">
          <div class="flex min-w-0 items-center gap-3">
            <div class="h-11 w-11 flex-shrink-0 overflow-hidden rounded-xl bg-white/90 p-1 shadow-xl shadow-slate-300/40 ring-1 ring-slate-900/10 dark:shadow-blue-950/30 dark:ring-white/30">
              <img :src="siteLogo || '/let-build-logo-256.png'" alt="Logo" class="h-full w-full object-contain" width="256" height="256" decoding="async" />
            </div>
            <span class="truncate text-sm font-semibold tracking-wide text-slate-950/95 dark:text-white/95 sm:text-base">
              {{ siteName }}
            </span>
          </div>

          <div class="flex items-center gap-2 sm:gap-3">
            <div class="rounded-full border border-slate-900/10 bg-white/70 backdrop-blur-md dark:border-white/15 dark:bg-slate-950/35">
              <LocaleSwitcher />
            </div>

            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex h-10 w-10 items-center justify-center rounded-full border border-slate-900/10 bg-white/70 text-slate-700 backdrop-blur-md transition hover:border-slate-900/20 hover:bg-white hover:text-slate-950 dark:border-white/15 dark:bg-slate-950/35 dark:text-white/80 dark:hover:border-white/30 dark:hover:bg-white/15 dark:hover:text-white"
              :title="t('home.viewDocs')"
            >
              <Icon name="book" size="md" />
            </a>

            <router-link
              v-if="showModelPlazaEntry"
              to="/model-plaza"
              class="inline-flex h-10 items-center gap-1.5 rounded-full border border-slate-900/10 bg-white/70 px-3 text-sm text-slate-700 backdrop-blur-md transition hover:bg-white dark:border-white/15 dark:bg-slate-950/35 dark:text-white/80 dark:hover:bg-white/15"
              :title="t('nav.modelPlaza')"
            >
              <Icon name="grid" size="sm" />
              <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
            </router-link>

            <button
              @click="toggleTheme"
              class="inline-flex h-10 w-10 items-center justify-center rounded-full border border-slate-900/10 bg-white/70 text-slate-700 backdrop-blur-md transition hover:border-slate-900/20 hover:bg-white hover:text-slate-950 dark:border-white/15 dark:bg-slate-950/35 dark:text-white/80 dark:hover:border-white/30 dark:hover:bg-white/15 dark:hover:text-white"
              :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            >
              <Icon v-if="isDark" name="sun" size="md" />
              <Icon v-else name="moon" size="md" />
            </button>

            <router-link
              v-if="isAuthenticated"
              :to="dashboardPath"
              class="inline-flex h-10 items-center gap-2 rounded-full bg-slate-950 px-2.5 py-1 text-sm font-semibold text-white shadow-lg shadow-slate-300/30 transition hover:bg-slate-800 dark:bg-white dark:text-slate-950 dark:shadow-blue-950/20 dark:hover:bg-blue-50 sm:px-4"
            >
              <span
                class="flex h-6 w-6 items-center justify-center rounded-full bg-gradient-to-br from-blue-500 to-cyan-400 text-xs font-bold text-white"
              >
                {{ userInitial }}
              </span>
              <span class="hidden sm:inline">{{ t('home.dashboard') }}</span>
            </router-link>
            <router-link
              v-else
              to="/login"
              class="inline-flex h-10 items-center rounded-full bg-slate-950 px-4 text-sm font-semibold text-white shadow-lg shadow-slate-300/30 transition hover:bg-slate-800 dark:bg-white dark:text-slate-950 dark:shadow-blue-950/20 dark:hover:bg-blue-50"
            >
              {{ t('home.login') }}
            </router-link>
          </div>
        </nav>
      </header>

      <main class="relative z-10 mx-auto flex min-h-[calc(88vh-76px)] max-w-7xl flex-col justify-center px-5 pb-16 pt-10 sm:px-6 lg:px-8">
        <div class="max-w-3xl">
          <div class="mb-5 inline-flex items-center gap-2 rounded-full border border-cyan-600/20 bg-white/70 px-4 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-cyan-800 backdrop-blur-md dark:border-cyan-200/25 dark:bg-cyan-950/35 dark:text-cyan-100">
            <span class="h-2 w-2 rounded-full bg-cyan-300 shadow-[0_0_18px_rgba(103,232,249,0.95)]"></span>
            {{ t('home.heroKicker') }}
          </div>

          <h1 class="max-w-4xl text-5xl font-black leading-[0.95] tracking-normal text-slate-950 sm:text-6xl dark:text-white dark:drop-shadow-2xl lg:text-7xl">
            {{ siteName }}
          </h1>

          <p class="mt-6 max-w-2xl text-xl font-semibold leading-8 text-slate-900 dark:text-white sm:text-2xl">
            {{ siteSubtitle }}
          </p>
          <p class="mt-4 max-w-2xl text-base leading-8 text-slate-700 dark:text-blue-50/85 sm:text-lg">
            {{ t('home.heroDescription') }}
          </p>

          <div class="mt-8 flex flex-col gap-3 sm:flex-row">
            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="inline-flex items-center justify-center rounded-full bg-slate-950 px-7 py-3 text-base font-bold text-white shadow-xl shadow-slate-300/40 transition hover:bg-slate-800 dark:bg-cyan-300 dark:text-slate-950 dark:shadow-cyan-500/25 dark:hover:bg-cyan-200"
            >
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
              <Icon name="arrowRight" size="md" class="ml-2" :stroke-width="2" />
            </router-link>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center justify-center rounded-full border border-slate-900/10 bg-white/70 px-7 py-3 text-base font-semibold text-slate-900 backdrop-blur-md transition hover:border-slate-900/20 hover:bg-white dark:border-white/20 dark:bg-white/10 dark:text-white dark:hover:border-white/35 dark:hover:bg-white/18"
            >
              {{ t('home.docs') }}
              <Icon name="externalLink" size="sm" class="ml-2" :stroke-width="2" />
            </a>
          </div>

          <div class="mt-8 flex flex-wrap gap-3">
            <span
              v-for="tag in heroTags"
              :key="tag.label"
              class="inline-flex items-center gap-2 rounded-full border border-slate-900/10 bg-white/75 px-4 py-2 text-sm font-medium text-slate-700 backdrop-blur-md dark:border-white/15 dark:bg-slate-950/35 dark:text-blue-50"
            >
              <Icon :name="tag.icon" size="sm" class="text-cyan-700 dark:text-cyan-200" />
              {{ tag.label }}
            </span>
          </div>
        </div>
      </main>
    </section>

    <section class="border-y border-slate-200 bg-white px-5 py-6 dark:border-white/10 dark:bg-slate-950 sm:px-6 lg:px-8">
      <div class="mx-auto grid max-w-7xl gap-4 sm:grid-cols-3">
        <div
          v-for="stat in heroStats"
          :key="stat.label"
          class="rounded-lg border border-slate-200 bg-slate-50 px-5 py-4 dark:border-white/10 dark:bg-white/[0.04]"
        >
          <div class="text-2xl font-black text-cyan-700 dark:text-cyan-200">{{ stat.value }}</div>
          <div class="mt-1 text-sm text-slate-600 dark:text-slate-300">{{ stat.label }}</div>
        </div>
      </div>
    </section>

    <section class="bg-slate-50 px-5 py-16 dark:bg-slate-950 sm:px-6 lg:px-8 lg:py-20">
      <div class="mx-auto max-w-7xl">
        <div class="max-w-3xl">
          <p class="text-sm font-semibold uppercase tracking-[0.2em] text-cyan-700 dark:text-cyan-300">
            {{ t('home.solutions.title') }}
          </p>
          <h2 class="mt-3 text-3xl font-black tracking-normal text-slate-950 dark:text-white sm:text-4xl">
            {{ t('home.solutions.subtitle') }}
          </h2>
        </div>

        <div class="mt-10 grid gap-5 md:grid-cols-3">
          <article
            v-for="feature in featureCards"
            :key="feature.title"
            class="rounded-lg border border-slate-200 bg-white p-6 shadow-sm transition hover:-translate-y-0.5 hover:border-cyan-300/70 hover:shadow-lg hover:shadow-slate-200/70 dark:border-white/10 dark:bg-white/[0.04] dark:shadow-none dark:hover:border-cyan-300/35 dark:hover:bg-white/[0.07] dark:hover:shadow-none"
          >
            <div
              class="mb-5 flex h-12 w-12 items-center justify-center rounded-lg"
              :class="feature.iconClass"
            >
              <Icon :name="feature.icon" size="lg" class="text-white" />
            </div>
            <h3 class="text-lg font-bold text-slate-950 dark:text-white">{{ feature.title }}</h3>
            <p class="mt-3 text-sm leading-7 text-slate-600 dark:text-slate-300">{{ feature.description }}</p>
          </article>
        </div>
      </div>
    </section>

    <section class="bg-slate-100 px-5 py-16 text-slate-950 dark:bg-slate-900 dark:text-white sm:px-6 lg:px-8 lg:py-20">
      <div class="mx-auto grid max-w-7xl gap-10 lg:grid-cols-[0.95fr_1.05fr] lg:items-center">
        <div class="overflow-hidden rounded-lg shadow-2xl shadow-slate-300/60 ring-1 ring-slate-200 dark:shadow-slate-950/40 dark:ring-white/10">
          <picture>
            <source
              type="image/webp"
              srcset="/frontpage/1-720.webp 720w, /frontpage/1-1080.webp 1080w, /frontpage/1-1536.webp 1536w"
              sizes="(min-width: 1024px) 48vw, 100vw"
            />
            <img
              src="/frontpage/1.png"
              alt=""
              class="aspect-[3/2] h-full w-full object-cover"
              width="1536"
              height="1024"
              loading="lazy"
              decoding="async"
            />
          </picture>
        </div>

        <div>
          <p class="text-sm font-semibold uppercase tracking-[0.2em] text-blue-700 dark:text-cyan-300">
            {{ t('home.operations.eyebrow') }}
          </p>
          <h2 class="mt-3 text-3xl font-black tracking-normal sm:text-4xl">
            {{ t('home.operations.title') }}
          </h2>
          <p class="mt-5 text-base leading-8 text-slate-600 dark:text-slate-300">
            {{ t('home.operations.description') }}
          </p>

          <div class="mt-8 grid gap-4 sm:grid-cols-2">
            <div
              v-for="item in operationItems"
              :key="item.title"
              class="rounded-lg border border-slate-200 bg-white p-5 shadow-sm dark:border-white/10 dark:bg-white/[0.04] dark:shadow-none"
            >
              <Icon :name="item.icon" size="lg" class="text-blue-600 dark:text-cyan-300" />
              <h3 class="mt-4 text-base font-bold">{{ item.title }}</h3>
              <p class="mt-2 text-sm leading-6 text-slate-600 dark:text-slate-300">{{ item.description }}</p>
            </div>
          </div>
        </div>
      </div>
    </section>

    <section class="bg-white px-5 py-16 dark:bg-slate-950 sm:px-6 lg:px-8 lg:py-20">
      <div class="mx-auto grid max-w-7xl gap-10 lg:grid-cols-[1fr_0.95fr] lg:items-center">
        <div>
          <p class="text-sm font-semibold uppercase tracking-[0.2em] text-cyan-700 dark:text-cyan-300">
            {{ t('home.scale.eyebrow') }}
          </p>
          <h2 class="mt-3 text-3xl font-black tracking-normal text-slate-950 dark:text-white sm:text-4xl">
            {{ t('home.scale.title') }}
          </h2>
          <p class="mt-5 text-base leading-8 text-slate-600 dark:text-slate-300">
            {{ t('home.scale.description') }}
          </p>

          <div class="mt-8 space-y-3">
            <div
              v-for="point in scalePoints"
              :key="point"
              class="flex items-start gap-3 rounded-lg border border-slate-200 bg-slate-50 p-4 text-sm leading-6 text-slate-700 dark:border-white/10 dark:bg-white/[0.04] dark:text-slate-200"
            >
              <Icon name="checkCircle" size="md" class="mt-0.5 flex-shrink-0 text-cyan-700 dark:text-cyan-300" />
              <span>{{ point }}</span>
            </div>
          </div>
        </div>

        <div class="overflow-hidden rounded-lg shadow-2xl shadow-slate-300/60 ring-1 ring-slate-200 dark:shadow-cyan-950/40 dark:ring-white/10">
          <picture>
            <source
              type="image/webp"
              srcset="/frontpage/3-720.webp 720w, /frontpage/3-1080.webp 1080w, /frontpage/3-1440.webp 1440w"
              sizes="(min-width: 1024px) 46vw, 100vw"
            />
            <img
              src="/frontpage/3.png"
              alt=""
              class="aspect-[16/9] h-full w-full object-cover"
              width="2560"
              height="1440"
              loading="lazy"
              decoding="async"
            />
          </picture>
        </div>
      </div>
    </section>

    <section class="bg-slate-100 px-5 py-14 text-slate-950 dark:bg-slate-900 dark:text-white sm:px-6 lg:px-8">
      <div class="mx-auto max-w-7xl">
        <div class="flex flex-col gap-6 md:flex-row md:items-end md:justify-between">
          <div>
            <h2 class="text-2xl font-black sm:text-3xl">{{ t('home.providers.title') }}</h2>
            <p class="mt-2 text-sm text-slate-600 dark:text-slate-300">{{ t('home.providers.description') }}</p>
          </div>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex items-center justify-center rounded-full bg-slate-950 px-6 py-3 text-sm font-bold text-white transition hover:bg-slate-800 dark:bg-cyan-300 dark:text-slate-950 dark:hover:bg-cyan-200"
          >
            {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
            <Icon name="arrowRight" size="sm" class="ml-2" :stroke-width="2" />
          </router-link>
        </div>

        <div class="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-[repeat(auto-fit,minmax(220px,1fr))]">
          <div
            v-for="provider in providerBadges"
            :key="provider.name"
            class="group relative overflow-hidden rounded-lg border bg-white p-4 shadow-sm transition duration-200 dark:bg-white/[0.04] dark:shadow-none"
            :class="provider.soon
              ? 'border-slate-200/80 text-slate-500 opacity-75 dark:border-white/10 dark:text-slate-400'
              : 'border-slate-200 text-slate-950 hover:-translate-y-0.5 hover:border-slate-300 hover:shadow-lg hover:shadow-slate-200/70 dark:border-white/10 dark:text-white dark:hover:border-cyan-300/35 dark:hover:bg-white/[0.07] dark:hover:shadow-none'"
          >
            <div
              class="absolute inset-x-0 top-0 h-1"
              :class="provider.soon ? 'bg-slate-200 dark:bg-slate-600' : provider.badgeClass"
            ></div>
            <div class="flex items-start justify-between gap-3">
              <span
                class="flex h-10 w-10 items-center justify-center rounded-lg text-xs font-black text-white shadow-sm transition group-hover:scale-105"
                :class="provider.badgeClass"
              >
                {{ provider.initial }}
              </span>
              <span
                class="inline-flex items-center gap-1.5 rounded-full px-2 py-1 text-[11px] font-bold"
                :class="provider.soon ? 'bg-slate-100 text-slate-500 dark:bg-white/10 dark:text-slate-300' : 'bg-emerald-50 text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-200'"
              >
                <span
                  class="h-1.5 w-1.5 rounded-full"
                  :class="provider.soon ? 'bg-slate-400' : 'bg-emerald-500'"
                ></span>
                {{ provider.soon ? t('home.providers.soon') : t('home.providers.supported') }}
              </span>
            </div>
            <div class="mt-4">
              <p class="text-base font-black tracking-normal">{{ provider.name }}</p>
              <p class="mt-1 text-xs font-medium text-slate-500 dark:text-slate-400">{{ provider.caption }}</p>
            </div>
          </div>
        </div>
      </div>
    </section>

    <footer class="border-t border-slate-200 bg-white px-5 py-8 dark:border-white/10 dark:bg-slate-950 sm:px-6 lg:px-8">
      <div
        class="mx-auto flex max-w-7xl flex-col items-center justify-center gap-4 text-center"
      >
        <p class="text-sm text-slate-500 dark:text-slate-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-slate-500 transition hover:text-slate-950 dark:text-slate-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { displaySiteName } from '@/utils/siteName'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t, locale } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => displaySiteName(appStore.cachedPublicSettings?.site_name || appStore.siteName, locale.value))
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => {
  const subtitle = appStore.cachedPublicSettings?.site_subtitle?.trim()
  if (!subtitle || subtitle === 'Subscription to API Conversion Platform') {
    return t('home.heroSubtitle')
  }
  return subtitle
})
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))
const modelPlazaRequiresAuth = computed(() => appStore.cachedPublicSettings?.model_plaza_require_auth === true)
const showModelPlazaEntry = computed(() => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value))

const iconName = <T extends string>(name: T) => name as T

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

const heroTags = computed(() => [
  { icon: iconName('swap'), label: t('home.tags.subscriptionToApi') },
  { icon: iconName('shield'), label: t('home.tags.stickySession') },
  { icon: iconName('chart'), label: t('home.tags.realtimeBilling') },
])

const heroStats = computed(() => [
  { value: t('home.stats.gateway.value'), label: t('home.stats.gateway.label') },
  { value: t('home.stats.routing.value'), label: t('home.stats.routing.label') },
  { value: t('home.stats.ops.value'), label: t('home.stats.ops.label') },
])

const featureCards = computed(() => [
  {
    icon: iconName('server'),
    iconClass: 'bg-blue-600 shadow-lg shadow-blue-900/30',
    title: t('home.features.unifiedGateway'),
    description: t('home.features.unifiedGatewayDesc'),
  },
  {
    icon: iconName('database'),
    iconClass: 'bg-cyan-600 shadow-lg shadow-cyan-900/30',
    title: t('home.features.multiAccount'),
    description: t('home.features.multiAccountDesc'),
  },
  {
    icon: iconName('chart'),
    iconClass: 'bg-amber-500 shadow-lg shadow-amber-900/30',
    title: t('home.features.balanceQuota'),
    description: t('home.features.balanceQuotaDesc'),
  },
])

const operationItems = computed(() => [
  {
    icon: iconName('key'),
    title: t('home.operations.items.keys.title'),
    description: t('home.operations.items.keys.description'),
  },
  {
    icon: iconName('shield'),
    title: t('home.operations.items.routing.title'),
    description: t('home.operations.items.routing.description'),
  },
  {
    icon: iconName('shield'),
    title: t('home.operations.items.sessions.title'),
    description: t('home.operations.items.sessions.description'),
  },
  {
    icon: iconName('chartBar'),
    title: t('home.operations.items.billing.title'),
    description: t('home.operations.items.billing.description'),
  },
])

const scalePoints = computed(() => [
  t('home.scale.points.monitoring'),
  t('home.scale.points.failover'),
  t('home.scale.points.billing'),
])

const providerBadges = computed(() => [
  {
    name: 'GPT',
    initial: 'G',
    badgeClass: 'bg-emerald-600',
    caption: t('home.providers.captions.gpt'),
    soon: false,
  },
  {
    name: t('home.providers.gemini'),
    initial: 'G',
    badgeClass: 'bg-blue-600',
    caption: t('home.providers.captions.gemini'),
    soon: false,
  },
  {
    name: t('home.providers.more'),
    initial: '+',
    badgeClass: 'bg-slate-500',
    caption: t('home.providers.captions.more'),
    soon: true,
  },
])

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>
