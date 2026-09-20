<script setup lang="ts">
withDefaults(defineProps<{ name: string; size?: number }>(), { size: 20 })
const paths: Record<string, string> = {
  review: 'M3 5h7a3 3 0 0 1 3 3v13a4 4 0 0 0-4-3H3V5Zm18 0h-5a3 3 0 0 0-3 3v13a4 4 0 0 1 4-3h4V5Z',
  game: 'M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2ZM8 8h.01M16 8h.01M12 12h.01M8 16h.01M16 16h.01',
  group:
    'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8ZM22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75',
  plus: 'M12 5v14M5 12h14',
  down: 'm6 9 6 6 6-6',
  arrow: 'M5 12h14m-5-5 5 5-5 5',
  back: 'm15 18-6-6 6-6',
  search: 'M21 21l-5-5M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16Z',
  filter: 'M4 7h16M7 12h10M10 17h4',
  clock: 'M12 8v4l3 2M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20Z',
  pin: 'M20 10c0 6-8 12-8 12S4 16 4 10a8 8 0 1 1 16 0ZM12 13a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z',
  cup: 'M8 3h8v5a4 4 0 0 1-8 0V3ZM8 5H4v2a4 4 0 0 0 4 4m8-6h4v2a4 4 0 0 1-4 4M12 12v7M8 21h8M9 19h6',
  photo: 'M4 4h16v16H4V4Zm0 12 5-5 4 4 3-3 4 4M15 8h.01',
  check: 'm5 12 4 4L19 6',
  close: 'm6 6 12 12M6 18 18 6',
  calendar: 'M4 5h16v16H4V5ZM8 3v4m8-4v4M4 10h16',
  edit: 'm15 5 4 4M4 20l4-1L20 7a2 2 0 0 0-4-4L4 15v5Z',
  link: 'm9 15 6-6M7 17l-1 1a4 4 0 0 1-6-6l5-5a4 4 0 0 1 6 0m2 10a4 4 0 0 0 6 0l5-5a4 4 0 0 0-6-6l-1 1',
  heart: 'M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.8-8.6a5.5 5.5 0 0 0 0-7.8Z',
  more: 'M5 12h.01M12 12h.01M19 12h.01',
  lock: 'M6 10h12v11H6V10Zm3 0V6a3 3 0 0 1 6 0v4M12 14v3',
  logout: 'M9 4H4v16h5M10 12h11m-4-4 4 4-4 4',
  bell: 'M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4',
  archive: 'M4 7h16v14H4V7Zm-1-4h18v4H3V3Zm6 8h6',
  download: 'M12 3v12m-5-5 5 5 5-5M5 21h14',
}
</script>
<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.65"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="paths[name] ?? paths.game" />
  </svg>
</template>
