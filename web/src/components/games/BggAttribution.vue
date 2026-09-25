<script setup lang="ts">
// Official BoardGameGeek logo: https://drive.google.com/file/d/1iQTdLUoxa8NM8T1tJEJj1GA6iPzpZ-AR/view
import logo from '@/assets/powered-by-bgg-rgb.svg'
</script>

<template>
  <a class="bgg-attribution" href="https://boardgamegeek.com/" target="_blank" rel="noopener noreferrer">
    <img :src="logo" width="342" height="76" alt="Powered by BGG" />
    <span>部分桌游资料来自 BoardGameGeek</span>
  </a>
</template>

<style scoped>
.bgg-attribution {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  max-width: 100%;
  color: #74776d;
  font-size: 9px;
  line-height: 1.4;
  text-align: center;
  text-decoration: none;
}
.bgg-attribution img {
  display: block;
  width: 130px;
  max-width: 100%;
  height: auto;
}
.bgg-attribution:focus-visible {
  outline: 2px solid var(--d-accent);
  outline-offset: 4px;
  border-radius: 2px;
}
</style>
