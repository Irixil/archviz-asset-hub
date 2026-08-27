<script lang="ts">
  import { onMount } from 'svelte'
  import { m } from '$lib/paraglide/messages'

  let svgEl: SVGSVGElement
  let legendEl: HTMLDivElement

  onMount(() => {
    const NS = 'http://www.w3.org/2000/svg'
    const CX = 480,
      CY = 234,
      VW = 960,
      VH = 468
    const FLOWER_R = 162
    const T_MORPH_START = 4000,
      T_MORPH_END = 5700

    const SRC = [
      {
        label: m.hp_diag_src_email_label(),
        sub: m.hp_diag_src_email_sub(),
        c1: '#D4A017',
        b1: '#FCD34D',
        c2: '#7c6db3',
      },
      {
        label: m.hp_diag_src_s3_label(),
        sub: m.hp_diag_src_s3_sub(),
        c1: '#0D9488',
        b1: '#2DD4BF',
        c2: '#7c6db3',
      },
      {
        label: m.hp_diag_src_sftp_label(),
        sub: m.hp_diag_src_sftp_sub(),
        c1: '#9333EA',
        b1: '#D8B4FE',
        c2: '#7c6db3',
      },
      {
        label: m.hp_diag_src_webdav_label(),
        sub: m.hp_diag_src_webdav_sub(),
        c1: '#DC2626',
        b1: '#FCA5A5',
        c2: '#7c6db3',
      },
      {
        label: m.hp_diag_src_imap_label(),
        sub: m.hp_diag_src_imap_sub(),
        c1: '#059669',
        b1: '#6EE7B7',
        c2: '#7c6db3',
      },
    ]
    const OUT = [
      {
        label: m.hp_diag_out_web_label(),
        sub: m.hp_diag_out_web_sub(),
        c1: '#2563EB',
        b1: '#93C5FD',
        c2: '#4a4870',
      },
      {
        label: m.hp_diag_out_cdn_label(),
        sub: m.hp_diag_out_cdn_sub(),
        c1: '#EA580C',
        b1: '#FED7AA',
        c2: '#4a4870',
      },
      {
        label: m.hp_diag_out_shares_label(),
        sub: m.hp_diag_out_shares_sub(),
        c1: '#7C3AED',
        b1: '#DDD6FE',
        c2: '#4a4870',
      },
      {
        label: m.hp_diag_out_downloads_label(),
        sub: m.hp_diag_out_downloads_sub(),
        c1: '#DB2777',
        b1: '#FBCFE8',
        c2: '#4a4870',
      },
      {
        label: m.hp_diag_out_integrations_label(),
        sub: m.hp_diag_out_integrations_sub(),
        c1: '#0891B2',
        b1: '#A5F3FC',
        c2: '#4a4870',
      },
    ]
    const CHART_Y = [96, 168, 240, 312, 384]

    function flowerPos(slot: number) {
      const a = ((-90 + slot * 36) * Math.PI) / 180
      return { x: CX + FLOWER_R * Math.cos(a), y: CY + FLOWER_R * Math.sin(a) }
    }

    type NodeDef = {
      label: string
      sub: string
      c1: string
      b1: string
      c2: string
      type: string
      idx: number
      p1: { x: number; y: number }
      p2: { x: number; y: number }
      el?: SVGGElement
      p1s?: SVGGElement
      p2s?: SVGGElement
    }

    const allNodes: NodeDef[] = [
      ...SRC.map((d, i) => ({
        ...d,
        type: 'src',
        idx: i,
        p1: flowerPos(i * 2),
        p2: { x: 92, y: CHART_Y[i] },
      })),
      ...OUT.map((d, i) => ({
        ...d,
        type: 'out',
        idx: i,
        p1: flowerPos(i * 2 + 1),
        p2: { x: 868, y: CHART_Y[i] },
      })),
    ]

    function mk(
      tag: string,
      attrs: Record<string, unknown> = {},
      text?: string
    ): SVGElement {
      const e = document.createElementNS(NS, tag)
      for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, String(v))
      if (text !== undefined) e.textContent = text
      return e
    }
    function lerp(a: number, b: number, t: number) {
      return a + (b - a) * t
    }
    function ease(t: number) {
      return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2
    }
    function clamp(v: number, lo: number, hi: number) {
      return Math.max(lo, Math.min(hi, v))
    }

    function buildDotGrid() {
      const g = svgEl.getElementById('dot-grid')
      for (let x = 30; x < VW; x += 30)
        for (let y = 20; y < VH; y += 30)
          g!.appendChild(
            mk('circle', {
              cx: x,
              cy: y,
              r: 0.9,
              fill: '#8890b8',
              opacity: 0.22,
            })
          )
    }

    function buildHubRingDiamonds() {
      const g = svgEl.getElementById('hub-ring-diamonds')
      for (let i = 0; i < 8; i++) {
        const a = (i * 45 * Math.PI) / 180,
          r = 67
        const x = CX + r * Math.cos(a),
          y = CY + r * Math.sin(a)
        g!.appendChild(
          mk('polygon', {
            points: `${x},${y - 4} ${x + 4},${y} ${x},${y + 4} ${x - 4},${y}`,
            fill: '#c4a6ff',
            opacity: 0.7,
          })
        )
      }
    }

    const PILL_LABELS = [
      m.hp_diag_pill_thumbnail(),
      m.hp_diag_pill_watermark(),
      m.hp_diag_pill_convert(),
      m.hp_diag_pill_resize(),
      m.hp_diag_pill_crop(),
      m.hp_diag_pill_transcode(),
      m.hp_diag_pill_normalize(),
      m.hp_diag_pill_extract_audio(),
      m.hp_diag_pill_bg_remove(),
    ]
    const PILL_N = PILL_LABELS.length
    const PILL_CYCLE = 3200
    const PILL_FADE = 280

    let pillTexts: SVGTextElement[] = []
    let pillRects: SVGRectElement[] = []
    let pillWindowStart = 0
    let replaceSlot = 0
    let pillTimer = PILL_CYCLE
    let pillFading = false
    let pillFadeT = 0

    function buildPills() {
      const g = svgEl.getElementById('pills')
      // 2×2 grid in hub phase 2: centered horizontally on CX=480,
      // vertically between separator (y=219) and VARIANTS label (y=278)
      // grid center at y=248: topY=226, botY=248
      const PW = 80,
        PH = 16,
        GAP_X = 8,
        GAP_Y = 6
      const GRID_CY = 248
      const leftX = CX - PW - GAP_X / 2
      const rightX = CX + GAP_X / 2
      const topY = GRID_CY - PH - GAP_Y / 2
      const botY = GRID_CY + GAP_Y / 2
      ;(
        [
          [leftX, topY],
          [rightX, topY],
          [leftX, botY],
          [rightX, botY],
        ] as [number, number][]
      ).forEach(([x, y], i) => {
        const pg = mk('g', { class: 'dm-pill' })
        ;(pg as unknown as HTMLElement).style.animationDelay = `${i * 0.55}s`
        const rect = mk('rect', {
          x,
          y,
          width: PW,
          height: PH,
          rx: 3.5,
          fill: '#1e1b3a',
          stroke: '#9580ff',
          'stroke-width': 0.8,
          'stroke-opacity': 0.7,
        }) as SVGRectElement
        pg.appendChild(rect)
        pillRects.push(rect)
        const txt = mk(
          'text',
          {
            x: x + PW / 2,
            y: y + PH - 3.5,
            'text-anchor': 'middle',
            fill: '#c4b5fd',
            'font-size': 8.5,
            'font-family': 'monospace',
            'letter-spacing': '.02em',
          },
          PILL_LABELS[i]
        ) as SVGTextElement
        pg.appendChild(txt)
        pillTexts.push(txt)
        g!.appendChild(pg)
      })
    }

    function buildFlowerPaths() {
      const g = svgEl.getElementById('p1paths')
      const order = [0, 5, 1, 6, 2, 7, 3, 8, 4, 9].map((i) => allNodes[i])
      order.forEach((n) => {
        g!.appendChild(
          mk('line', {
            x1: CX,
            y1: CY,
            x2: n.p1.x.toFixed(1),
            y2: n.p1.y.toFixed(1),
            stroke: n.c1,
            'stroke-width': 1,
            'stroke-opacity': 0.55,
          })
        )
      })
      g!.appendChild(
        mk('polygon', {
          points: order
            .map((n) => `${n.p1.x.toFixed(1)},${n.p1.y.toFixed(1)}`)
            .join(' '),
          fill: 'none',
          stroke: '#c4a6ff',
          'stroke-width': 0.9,
          'stroke-opacity': 0.35,
        })
      )
      g!.appendChild(
        mk('polygon', {
          points: allNodes
            .slice(0, 5)
            .map((n) => `${n.p1.x.toFixed(1)},${n.p1.y.toFixed(1)}`)
            .join(' '),
          fill: 'none',
          stroke: '#F59E0B',
          'stroke-width': 0.8,
          'stroke-opacity': 0.22,
        })
      )
      g!.appendChild(
        mk('polygon', {
          points: allNodes
            .slice(5)
            .map((n) => `${n.p1.x.toFixed(1)},${n.p1.y.toFixed(1)}`)
            .join(' '),
          fill: 'none',
          stroke: '#9580ff',
          'stroke-width': 0.8,
          'stroke-opacity': 0.22,
        })
      )
      const starOrder = [0, 4, 8, 2, 6].map((i) => order[i])
      g!.appendChild(
        mk('polygon', {
          points: starOrder
            .map((n) => `${n.p1.x.toFixed(1)},${n.p1.y.toFixed(1)}`)
            .join(' '),
          fill: 'none',
          stroke: '#c4b5fd',
          'stroke-width': 0.5,
          'stroke-opacity': 0.15,
        })
      )
    }

    const sp: SVGPathElement[] = []
    const op: SVGPathElement[] = []

    function buildChartPaths() {
      const g = svgEl.getElementById('p2paths')
      CHART_Y.forEach((y, i) => {
        g!.appendChild(
          mk('path', {
            d: `M 154,${y} C 300,${y} 430,234 480,234`,
            stroke: '#7c6db3',
            'stroke-width': 1,
            'stroke-opacity': 0.13,
            fill: 'none',
          })
        )
        const di = mk('path', {
          d: `M 154,${y} C 300,${y} 430,234 480,234`,
          stroke: '#a99de0',
          'stroke-width': 1.4,
          fill: 'none',
          class: 'dm-dash-in',
        })
        ;(di as unknown as HTMLElement).style.animationDelay = `${-i * 0.3}s`
        g!.appendChild(di)
        g!.appendChild(
          mk('circle', {
            cx: 154,
            cy: y,
            r: 2.8,
            fill: '#7c6db3',
            opacity: 0.55,
          })
        )
        const spEl = mk('path', {
          d: `M 154,${y} C 300,${y} 430,234 480,234`,
          fill: 'none',
          stroke: 'none',
        }) as SVGPathElement
        g!.appendChild(spEl)
        sp.push(spEl)

        g!.appendChild(
          mk('path', {
            d: `M 480,234 C 530,234 680,${y} 806,${y}`,
            stroke: '#4a4870',
            'stroke-width': 1,
            'stroke-opacity': 0.13,
            fill: 'none',
          })
        )
        const dout = mk('path', {
          d: `M 480,234 C 530,234 680,${y} 806,${y}`,
          stroke: '#8880b8',
          'stroke-width': 1.4,
          fill: 'none',
          class: 'dm-dash-out',
        })
        ;(dout as unknown as HTMLElement).style.animationDelay = `${-i * 0.28}s`
        g!.appendChild(dout)
        g!.appendChild(
          mk('circle', {
            cx: 806,
            cy: y,
            r: 2.8,
            fill: '#4a4870',
            opacity: 0.55,
          })
        )
        const opEl = mk('path', {
          d: `M 480,234 C 530,234 680,${y} 806,${y}`,
          fill: 'none',
          stroke: 'none',
        }) as SVGPathElement
        g!.appendChild(opEl)
        op.push(opEl)
      })

      const sl = mk(
        'text',
        {
          id: 'lbl-src',
          x: 92,
          y: 52,
          'text-anchor': 'middle',
          fill: '#6e6a9a',
          'font-size': 9,
          'font-weight': 500,
          'letter-spacing': '.12em',
          opacity: 0,
        },
        m.hp_diag_lbl_ingress()
      )
      g!.appendChild(sl)
      const ol = mk(
        'text',
        {
          id: 'lbl-out',
          x: 868,
          y: 52,
          'text-anchor': 'middle',
          fill: '#6e6a9a',
          'font-size': 9,
          'font-weight': 500,
          'letter-spacing': '.12em',
          opacity: 0,
        },
        m.hp_diag_lbl_distribute()
      )
      g!.appendChild(ol)
    }

    function buildGemIcon(color: string, bright: string, letter: string) {
      const g = mk('g')
      g.appendChild(
        mk('polygon', {
          points: '0,-31 31,0 0,31 -31,0',
          fill: color,
          'fill-opacity': 0.82,
          stroke: bright,
          'stroke-width': 1.4,
          filter: 'url(#dm-gemglow)',
        })
      )
      g.appendChild(
        mk('polygon', {
          points: '0,-16 16,0 0,16 -16,0',
          fill: bright,
          'fill-opacity': 0.18,
          stroke: bright,
          'stroke-width': 0.7,
          'stroke-opacity': 0.6,
        })
      )
      ;[
        [-31, 0, -16, 0],
        [0, -31, 0, -16],
        [31, 0, 16, 0],
        [0, 31, 0, 16],
        [-22, -22, -11, -11],
        [22, -22, 11, -11],
        [22, 22, 11, 11],
        [-22, 22, -11, 11],
      ].forEach(([x1, y1, x2, y2]) =>
        g.appendChild(
          mk('line', {
            x1,
            y1,
            x2,
            y2,
            stroke: bright,
            'stroke-width': 0.5,
            'stroke-opacity': 0.45,
          })
        )
      )
      g.appendChild(
        mk(
          'text',
          {
            x: 0,
            y: 6,
            'text-anchor': 'middle',
            fill: 'white',
            'font-size': 13,
            'font-weight': 700,
            'font-family': 'monospace',
            opacity: 0.9,
          },
          letter
        )
      )
      return g
    }

    function buildChartIcon(node: NodeDef) {
      const g = mk('g')
      g.appendChild(
        mk('rect', {
          x: -62,
          y: -19,
          width: 124,
          height: 38,
          rx: 6,
          fill: '#1e1b3a',
          stroke: node.c2,
          'stroke-width': 1,
          'stroke-opacity': 0.55,
        })
      )
      g.appendChild(
        mk('circle', { cx: -44, cy: 0, r: 7, fill: node.c1, opacity: 0.85 })
      )
      g.appendChild(
        mk(
          'text',
          {
            x: -32,
            y: -5,
            fill: '#d4cff0',
            'font-size': 11,
            'font-weight': 500,
          },
          node.label
        )
      )
      g.appendChild(
        mk('text', { x: -32, y: 9, fill: '#6e6a9a', 'font-size': 9 }, node.sub)
      )
      return g
    }

    function buildNodes() {
      const container = svgEl.getElementById('dm-nodes')
      allNodes.forEach((node) => {
        const g = mk('g') as SVGGElement
        g.setAttribute(
          'transform',
          `translate(${node.p1.x.toFixed(1)},${node.p1.y.toFixed(1)})`
        )
        node.el = g

        const p1g = mk('g', { class: 'p1shape' }) as SVGGElement
        p1g.appendChild(buildGemIcon(node.c1, node.b1, node.label[0]))
        node.p1s = p1g
        g.appendChild(p1g)

        const p2g = mk('g', { class: 'p2shape', opacity: 0 }) as SVGGElement
        p2g.appendChild(buildChartIcon(node))
        node.p2s = p2g
        g.appendChild(p2g)

        container!.appendChild(g)
      })
    }

    function doMorph(t: number) {
      allNodes.forEach((n) => {
        const x = lerp(n.p1.x, n.p2.x, t),
          y = lerp(n.p1.y, n.p2.y, t)
        n.el!.setAttribute(
          'transform',
          `translate(${x.toFixed(2)},${y.toFixed(2)})`
        )
        n.p1s!.setAttribute('opacity', String(1 - t))
        n.p2s!.setAttribute('opacity', String(t))
      })
      svgEl.getElementById('p1bg')!.setAttribute('opacity', String(1 - t))
      svgEl.getElementById('p2bg')!.setAttribute('opacity', String(t))
      svgEl.getElementById('p1paths')!.setAttribute('opacity', String(1 - t))
      svgEl.getElementById('p2paths')!.setAttribute('opacity', String(t))
      svgEl.getElementById('dm-hub-p1')!.setAttribute('opacity', String(1 - t))
      svgEl.getElementById('dm-hub-p2')!.setAttribute('opacity', String(t))
      svgEl
        .getElementById('dm-fp')!
        .setAttribute('opacity', String(Math.max(0, 1 - t * 2)))
      const sl = svgEl.getElementById('lbl-src'),
        ol = svgEl.getElementById('lbl-out')
      if (sl) sl.setAttribute('opacity', String(t))
      if (ol) ol.setAttribute('opacity', String(t))
    }

    const fpContainer = svgEl.getElementById('dm-fp')!
    const ORBIT_COLORS = [
      '#FCD34D',
      '#2DD4BF',
      '#D8B4FE',
      '#FCA5A5',
      '#6EE7B7',
      '#93C5FD',
      '#FED7AA',
      '#DDD6FE',
      '#FBCFE8',
      '#c4b5fd',
    ]
    const orbiters = Array.from({ length: 14 }, (_, i) => ({
      angle: i * ((Math.PI * 2) / 14) + Math.random() * 0.5,
      r: 85 + (i % 3) * 22,
      speed: (0.0006 + (i % 5) * 0.00015) * (i % 2 ? 1 : -1),
      color: ORBIT_COLORS[i % 10],
      el: (() => {
        const c = mk('circle', {
          r: 2.8,
          fill: ORBIT_COLORS[i % 10],
          filter: 'url(#dm-pglow)',
          opacity: 0.8,
        })
        fpContainer.appendChild(c)
        return c
      })(),
    }))

    const cpContainer = svgEl.getElementById('dm-cp')!
    const cpPool: { tick: (dt: number) => void; alive: boolean }[] = []
    const sTimers = [0, 600, 1000, 1500, 1900]
    const oTimers = [300, 800, 1200, 1600, 2000]
    const sIv = [2100, 2700, 2350, 2850, 2450]
    const oIv = [2200, 2600, 2950, 2350, 2750]

    // svelte-ignore perf_avoid_nested_class
    class Particle {
      path: SVGPathElement
      len: number
      t: number
      speed: number
      alive: boolean
      el: SVGElement
      constructor(path: SVGPathElement, fill: string, r: number) {
        this.path = path
        this.len = path.getTotalLength()
        this.t = 0
        this.speed = 0.00022 + Math.random() * 0.00008
        this.alive = true
        this.el = mk('circle', { r, fill, filter: 'url(#dm-pglow)' })
        cpContainer.appendChild(this.el)
      }
      tick(dt: number) {
        this.t += this.speed * dt
        if (this.t >= 1) {
          this.el.remove()
          this.alive = false
          return
        }
        const pt = this.path.getPointAtLength(this.t * this.len)
        this.el.setAttribute('cx', String(pt.x))
        this.el.setAttribute('cy', String(pt.y))
        const a =
          this.t < 0.08
            ? this.t / 0.08
            : this.t > 0.86
              ? (1 - this.t) / 0.14
              : 1
        this.el.setAttribute('opacity', (a * 0.88).toFixed(3))
      }
    }

    let t0: number | null = null,
      tPrev: number | null = null,
      phase = 1
    let rafId: number

    function frame(ts: number) {
      if (!t0) {
        t0 = ts
        tPrev = ts
      }
      const elapsed = ts - t0,
        dt = Math.min(ts - tPrev!, 60)
      tPrev = ts

      if (elapsed < T_MORPH_START) {
        orbiters.forEach((o) => {
          o.angle += o.speed * dt
          o.el.setAttribute('cx', String(CX + o.r * Math.cos(o.angle)))
          o.el.setAttribute('cy', String(CY + o.r * Math.sin(o.angle)))
        })
      } else if (elapsed < T_MORPH_END) {
        phase = 2
        const p = ease(
          clamp((elapsed - T_MORPH_START) / (T_MORPH_END - T_MORPH_START), 0, 1)
        )
        doMorph(p)
        orbiters.forEach((o) => {
          o.angle += o.speed * dt * 0.3
          o.el.setAttribute('cx', String(CX + o.r * Math.cos(o.angle)))
          o.el.setAttribute('cy', String(CY + o.r * Math.sin(o.angle)))
        })
      } else {
        if (phase !== 3) {
          phase = 3
          doMorph(1)
        }
        for (let i = 0; i < 5; i++) {
          sTimers[i] -= dt
          if (sTimers[i] <= 0) {
            cpPool.push(new Particle(sp[i], '#a99de0', 3.2))
            sTimers[i] = sIv[i]
          }
          oTimers[i] -= dt
          if (oTimers[i] <= 0) {
            cpPool.push(new Particle(op[i], '#8880b8', 3.0))
            oTimers[i] = oIv[i]
          }
        }
        for (let i = cpPool.length - 1; i >= 0; i--) {
          cpPool[i].tick(dt)
          if (!cpPool[i].alive) cpPool.splice(i, 1)
        }

        // pill rotation
        pillTimer -= dt
        if (!pillFading && pillTimer <= 0) {
          pillFading = true
          pillFadeT = 0
        }
        if (pillFading) {
          pillFadeT += dt
          if (pillFadeT < PILL_FADE) {
            const a = 1 - pillFadeT / PILL_FADE
            pillTexts[replaceSlot].setAttribute('opacity', String(a))
            pillRects[replaceSlot].setAttribute('opacity', String(a))
          } else if (pillFadeT < PILL_FADE * 2) {
            if (pillFadeT - dt < PILL_FADE) {
              const nextLabel = PILL_LABELS[(pillWindowStart + 4) % PILL_N]
              pillTexts[replaceSlot].textContent = nextLabel
              pillWindowStart = (pillWindowStart + 1) % PILL_N
              replaceSlot = (replaceSlot + 1) % 4
            }
            const prevSlot = (replaceSlot + 3) % 4
            const a = (pillFadeT - PILL_FADE) / PILL_FADE
            pillTexts[prevSlot].setAttribute('opacity', String(a))
            pillRects[prevSlot].setAttribute('opacity', String(a))
          } else {
            const prevSlot = (replaceSlot + 3) % 4
            pillFading = false
            pillTimer = PILL_CYCLE
            pillTexts[prevSlot].setAttribute('opacity', '1')
            pillRects[prevSlot].setAttribute('opacity', '1')
          }
        }
      }
      rafId = requestAnimationFrame(frame)
    }

    buildDotGrid()
    buildHubRingDiamonds()
    buildPills()
    buildFlowerPaths()
    buildChartPaths()
    buildNodes()

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          observer.disconnect()
          rafId = requestAnimationFrame(frame)
        }
      },
      { threshold: 0.1 }
    )
    observer.observe(legendEl)

    return () => {
      observer.disconnect()
      cancelAnimationFrame(rafId)
    }
  })
</script>

<section class="dm-diagram-section">
  <div class="dm-diagram-inner">
    <div class="dm-diagram-header reveal">
      <span class="dm-eyebrow">{m.hp_diag_eyebrow()}</span>
      <h2 class="dm-headline">{m.hp_diag_headline()}</h2>
      <p class="dm-subline">{m.hp_diag_subline()}</p>
    </div>

    <div class="dm-svg-wrap reveal" style="--delay: 80ms">
      <svg
        bind:this={svgEl}
        viewBox="0 0 960 468"
        xmlns="http://www.w3.org/2000/svg"
        aria-label={m.hp_diag_aria()}
        role="img"
      >
        <defs>
          <pattern
            id="dm-arabesqueTile"
            x="0"
            y="0"
            width="60"
            height="60"
            patternUnits="userSpaceOnUse"
          >
            <rect width="60" height="60" fill="#0e0b1f" />
            <polygon
              points="18,0 42,0 60,18 60,42 42,60 18,60 0,42 0,18"
              fill="none"
              stroke="#c4a6ff"
              stroke-width=".9"
              opacity=".22"
            />
            <polygon
              points="30,18 32,27 39,21 33,28 42,30 33,32 39,39 32,33 30,42 28,33 21,39 27,32 18,30 27,28 21,21 28,27"
              fill="#9580ff"
              fill-opacity=".08"
              stroke="#9580ff"
              stroke-width=".6"
              opacity=".3"
            />
            <line
              x1="0"
              y1="0"
              x2="60"
              y2="60"
              stroke="#7c6db3"
              stroke-width=".4"
              opacity=".12"
            />
            <line
              x1="60"
              y1="0"
              x2="0"
              y2="60"
              stroke="#7c6db3"
              stroke-width=".4"
              opacity=".12"
            />
            <circle cx="0" cy="0" r="1.5" fill="#c4a6ff" opacity=".28" />
            <circle cx="60" cy="0" r="1.5" fill="#c4a6ff" opacity=".28" />
            <circle cx="0" cy="60" r="1.5" fill="#c4a6ff" opacity=".28" />
            <circle cx="60" cy="60" r="1.5" fill="#c4a6ff" opacity=".28" />
            <circle cx="30" cy="30" r="1.2" fill="#F59E0B" opacity=".3" />
          </pattern>

          <filter id="dm-pglow" x="-100%" y="-100%" width="300%" height="300%">
            <feGaussianBlur stdDeviation="2.2" result="b" />
            <feMerge
              ><feMergeNode in="b" /><feMergeNode in="SourceGraphic" /></feMerge
            >
          </filter>
          <filter id="dm-gemglow" x="-60%" y="-60%" width="220%" height="220%">
            <feGaussianBlur stdDeviation="5" result="b" />
            <feMerge
              ><feMergeNode in="b" /><feMergeNode in="SourceGraphic" /></feMerge
            >
          </filter>
          <filter id="dm-hubglow" x="-40%" y="-40%" width="180%" height="180%">
            <feGaussianBlur stdDeviation="8" result="b" />
            <feComposite in="SourceGraphic" in2="b" operator="over" />
          </filter>

          <radialGradient id="dm-hubAura" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stop-color="#5b4acf" />
            <stop offset="100%" stop-color="#5b4acf" stop-opacity="0" />
          </radialGradient>
          <radialGradient id="dm-hubAura2" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stop-color="#c4a6ff" />
            <stop offset="100%" stop-color="#c4a6ff" stop-opacity="0" />
          </radialGradient>
          <radialGradient id="dm-vignette" cx="50%" cy="50%" r="55%">
            <stop offset="0%" stop-color="#0e0b1f" stop-opacity="0" />
            <stop offset="100%" stop-color="#0e0b1f" stop-opacity=".7" />
          </radialGradient>
        </defs>

        <!-- Phase 1 background -->
        <g id="p1bg">
          <rect width="960" height="468" fill="url(#dm-arabesqueTile)" />
          <g class="dm-ring-cw">
            <circle
              cx="480"
              cy="234"
              r="108"
              fill="none"
              stroke="#F59E0B"
              stroke-width=".8"
              stroke-dasharray="3 9"
              opacity=".3"
            />
            <circle
              cx="480"
              cy="234"
              r="230"
              fill="none"
              stroke="#c4a6ff"
              stroke-width=".6"
              stroke-dasharray="2 12"
              opacity=".18"
            />
          </g>
          <g class="dm-ring-ccw">
            <circle
              cx="480"
              cy="234"
              r="148"
              fill="none"
              stroke="#9580ff"
              stroke-width=".7"
              stroke-dasharray="4 8"
              opacity=".22"
            />
            <circle
              cx="480"
              cy="234"
              r="190"
              fill="none"
              stroke="#c4a6ff"
              stroke-width=".5"
              stroke-dasharray="2 10"
              opacity=".15"
            />
          </g>
          <rect width="960" height="468" fill="url(#dm-vignette)" />
        </g>

        <!-- Phase 2 background -->
        <g id="p2bg" opacity="0">
          <rect width="960" height="468" fill="#13102a" />
          <g id="dot-grid" />
          <circle
            cx="480"
            cy="234"
            r="155"
            fill="url(#dm-hubAura)"
            class="dm-hub-aura"
          />
          <circle
            cx="480"
            cy="234"
            r="200"
            fill="url(#dm-hubAura)"
            class="dm-hub-aura-2"
          />
        </g>

        <!-- Phase 1 paths -->
        <g id="p1paths" />
        <!-- Phase 2 paths -->
        <g id="p2paths" opacity="0" />

        <!-- Hub phase 1 -->
        <g id="dm-hub-p1">
          <circle
            cx="480"
            cy="234"
            r="75"
            fill="#0e0b1f"
            fill-opacity=".95"
            stroke="#c4a6ff"
            stroke-width="1.5"
            filter="url(#dm-hubglow)"
          />
          <circle
            cx="480"
            cy="234"
            r="67"
            fill="none"
            stroke="#c4a6ff"
            stroke-width=".5"
            stroke-dasharray="3 5"
            opacity=".45"
          />
          <g id="hub-ring-diamonds" />
          <text
            x="480"
            y="240"
            text-anchor="middle"
            fill="#c4b5fd"
            font-size="17"
            font-weight="600"
            letter-spacing="-.01em">damask</text
          >
        </g>

        <!-- Hub phase 2 -->
        <g id="dm-hub-p2" opacity="0">
          <circle
            cx="480"
            cy="234"
            r="116"
            fill="none"
            stroke="#9580ff"
            stroke-width=".6"
            stroke-opacity=".18"
          />
          <circle
            cx="480"
            cy="234"
            r="102"
            fill="#1e1b3a"
            stroke="#9580ff"
            stroke-width="1.5"
          />
          <text
            x="480"
            y="210"
            text-anchor="middle"
            fill="#d4cff0"
            font-size="17"
            font-weight="600"
            letter-spacing="-.01em">damask</text
          >
          <line
            x1="396"
            y1="222"
            x2="564"
            y2="222"
            stroke="#9580ff"
            stroke-width=".6"
            stroke-opacity=".35"
          />
          <g id="pills" />
          <text
            x="480"
            y="278"
            text-anchor="middle"
            fill="#6e6a9a"
            font-size="7.5"
            letter-spacing=".12em">{m.hp_diag_lbl_variants()}</text
          >
        </g>

        <!-- Nodes -->
        <g id="dm-nodes" />
        <!-- Particles -->
        <g id="dm-fp" />
        <g id="dm-cp" />
      </svg>
    </div>

    <div class="dm-legend reveal" style="--delay: 140ms" bind:this={legendEl}>
      <div class="dm-legend-item">
        <div class="dm-leg-line dm-in"></div>
        <span>{m.hp_diag_legend_ingress()}</span>
      </div>
      <div class="dm-legend-item">
        <div class="dm-leg-pill"></div>
        <span>{m.hp_diag_legend_variants()}</span>
      </div>
      <div class="dm-legend-item">
        <div class="dm-leg-line dm-out"></div>
        <span>{m.hp_diag_legend_distrib()}</span>
      </div>
    </div>
  </div>
</section>

<style>
  .dm-diagram-section {
    background: var(--hp-dark-bg);
    padding: clamp(4rem, 8vw, 7rem) clamp(1.25rem, 4vw, 2.5rem);
  }

  .dm-diagram-inner {
    max-width: 1160px;
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    gap: 2rem;
  }

  .dm-diagram-header {
    text-align: center;
  }

  .dm-eyebrow {
    display: block;
    font-family: var(--hp-font-body);
    font-size: 0.75rem;
    font-weight: 600;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: oklch(55% 0.08 290);
    margin-bottom: 0.75rem;
  }

  .dm-headline {
    font-family: var(--hp-font-display);
    font-size: clamp(1.6rem, 3vw, 2.4rem);
    font-weight: 700;
    letter-spacing: -0.025em;
    color: var(--hp-dark-text);
    margin: 0 0 0.75rem;
  }

  .dm-subline {
    font-family: var(--hp-font-body);
    font-size: clamp(0.875rem, 1.5vw, 1rem);
    color: var(--hp-dark-muted);
    max-width: 520px;
    margin: 0 auto;
    line-height: 1.6;
  }

  .dm-svg-wrap {
    width: 100%;
    border-radius: 12px;
    overflow: hidden;
    border: 1px solid oklch(28% 0.06 290);
    background: #0e0b1f;
  }

  .dm-svg-wrap svg {
    width: 100%;
    height: auto;
    display: block;
    overflow: visible;
  }

  /* Keyframes */
  @keyframes dm-dashIn {
    to {
      stroke-dashoffset: -24;
    }
  }
  @keyframes dm-dashOut {
    to {
      stroke-dashoffset: -24;
    }
  }
  @keyframes dm-pillPulse {
    0%,
    100% {
      opacity: 0.45;
    }
    50% {
      opacity: 1;
    }
  }
  @keyframes dm-breathe {
    0%,
    100% {
      opacity: 0.08;
    }
    50% {
      opacity: 0.22;
    }
  }
  @keyframes dm-spinCW {
    to {
      transform: rotate(1turn);
    }
  }
  @keyframes dm-spinCCW {
    to {
      transform: rotate(-1turn);
    }
  }

  :global(.dm-dash-in) {
    stroke-dasharray: 5 7;
    animation: dm-dashIn 1.3s linear infinite;
  }
  :global(.dm-dash-out) {
    stroke-dasharray: 5 7;
    animation: dm-dashOut 1.3s linear infinite;
  }
  :global(.dm-pill) {
    animation: dm-pillPulse 2.2s ease-in-out infinite;
  }
  :global(.dm-hub-aura) {
    animation: dm-breathe 3.6s ease-in-out infinite;
  }
  :global(.dm-hub-aura-2) {
    animation: dm-breathe 3.6s ease-in-out infinite;
    animation-delay: -1.8s;
  }
  :global(.dm-ring-cw) {
    animation: dm-spinCW 90s linear infinite;
    transform-box: fill-box;
    transform-origin: center;
  }
  :global(.dm-ring-ccw) {
    animation: dm-spinCCW 70s linear infinite;
    transform-box: fill-box;
    transform-origin: center;
  }

  /* Legend */
  .dm-legend {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 2rem;
    flex-wrap: wrap;
  }

  .dm-legend-item {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-family: var(--hp-font-body);
    font-size: 0.6875rem;
    color: oklch(55% 0.06 290);
  }

  .dm-leg-line {
    width: 28px;
    height: 2px;
    border-radius: 1px;
    position: relative;
  }
  .dm-leg-line::after {
    content: '';
    position: absolute;
    right: 0;
    top: 50%;
    transform: translateY(-50%);
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
  .dm-leg-line.dm-in {
    background: linear-gradient(to right, transparent, #9580ff);
  }
  .dm-leg-line.dm-in::after {
    background: #a99de0;
  }
  .dm-leg-line.dm-out {
    background: linear-gradient(to right, transparent, #6e6a9a);
  }
  .dm-leg-line.dm-out::after {
    background: #8880b8;
  }
  .dm-leg-pill {
    width: 28px;
    height: 12px;
    border: 1px solid #9580ff;
    border-radius: 3px;
  }

  @media (prefers-reduced-motion: reduce) {
    :global(.dm-dash-in),
    :global(.dm-dash-out),
    :global(.dm-pill),
    :global(.dm-hub-aura),
    :global(.dm-hub-aura-2),
    :global(.dm-ring-cw),
    :global(.dm-ring-ccw) {
      animation: none;
    }
  }
</style>
