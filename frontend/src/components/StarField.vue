<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const canvasRef = ref<HTMLCanvasElement | null>(null)

interface Star {
  x: number
  y: number
  size: number
  baseAlpha: number
  twinkleSpeed: number
  twinklePhase: number
  r: number
  g: number
  b: number
}

const STAR_COUNT = 220
const PALETTES = [
  // blue-white (70%)
  { r: 200, g: 210, b: 255 },
  { r: 200, g: 210, b: 255 },
  { r: 200, g: 210, b: 255 },
  { r: 200, g: 210, b: 255 },
  { r: 200, g: 210, b: 255 },
  { r: 200, g: 210, b: 255 },
  { r: 200, g: 210, b: 255 },
  // warm white (20%)
  { r: 255, g: 248, b: 240 },
  { r: 255, g: 248, b: 240 },
  // pure white (10%)
  { r: 255, g: 255, b: 255 },
]

function generateStars(w: number, h: number): Star[] {
  const stars: Star[] = []
  for (let i = 0; i < STAR_COUNT; i++) {
    const tier = Math.random()
    let size: number
    let baseAlpha: number
    if (tier < 0.6) {
      // tiny — distant
      size = 0.5 + Math.random() * 0.4
      baseAlpha = 0.2 + Math.random() * 0.3
    } else if (tier < 0.9) {
      // medium
      size = 1.0 + Math.random() * 0.4
      baseAlpha = 0.4 + Math.random() * 0.3
    } else {
      // bright
      size = 1.5 + Math.random() * 0.5
      baseAlpha = 0.7 + Math.random() * 0.3
    }
    const col = PALETTES[Math.floor(Math.random() * PALETTES.length)]
    stars.push({
      x: Math.random() * w,
      y: Math.random() * h,
      size,
      baseAlpha,
      twinkleSpeed: 0.3 + Math.random() * 1.2,
      twinklePhase: Math.random() * Math.PI * 2,
      r: col.r,
      g: col.g,
      b: col.b,
    })
  }
  return stars
}

let stars: Star[] = []
let rafId = 0
let ctx: CanvasRenderingContext2D | null = null
let logicalW = 0
let logicalH = 0

function resize(w: number, h: number): void {
  const canvas = canvasRef.value
  if (!canvas || !ctx) return
  logicalW = w
  logicalH = h
  const dpr = window.devicePixelRatio || 1
  canvas.width = w * dpr
  canvas.height = h * dpr
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  stars = generateStars(w, h)
}

function draw(timestamp: number): void {
  if (!ctx) return
  const t = timestamp / 1000
  ctx.clearRect(0, 0, logicalW, logicalH)
  for (const star of stars) {
    const alpha = star.baseAlpha * (0.6 + 0.4 * Math.sin(t * star.twinkleSpeed + star.twinklePhase))
    ctx.globalAlpha = alpha
    ctx.fillStyle = `rgb(${star.r},${star.g},${star.b})`
    ctx.beginPath()
    ctx.arc(star.x, star.y, star.size / 2, 0, Math.PI * 2)
    ctx.fill()
  }
  rafId = requestAnimationFrame(draw)
}

let ro: ResizeObserver | null = null

onMounted(() => {
  const canvas = canvasRef.value
  if (!canvas) return
  ctx = canvas.getContext('2d')
  if (!ctx) return

  ro = new ResizeObserver((entries) => {
    const rect = entries[0]?.contentRect
    if (rect) resize(rect.width, rect.height)
  })
  ro.observe(canvas.parentElement ?? canvas)

  const rect = canvas.parentElement?.getBoundingClientRect() ?? canvas.getBoundingClientRect()
  resize(rect.width, rect.height)
  rafId = requestAnimationFrame(draw)
})

onUnmounted(() => {
  cancelAnimationFrame(rafId)
  ro?.disconnect()
})
</script>

<template>
  <canvas ref="canvasRef" class="star-field" aria-hidden="true" />
</template>

<style scoped>
.star-field {
  display: block;
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}
</style>
