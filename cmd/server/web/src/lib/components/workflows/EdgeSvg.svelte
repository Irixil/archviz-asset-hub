<script lang="ts">
  import type { WorkflowGraphEdge, WorkflowGraphNode } from '$lib/api/workflows'

  interface PendingEdge {
    from_node: string
    from_port: string
    x: number
    y: number
  }

  interface SchemaPort {
    id: string
    label: string
  }

  interface Props {
    edge?: WorkflowGraphEdge
    nodes: WorkflowGraphNode[]
    pending?: PendingEdge | null
    outputPortsMap?: Record<string, SchemaPort[]>
    inputPortsMap?: Record<string, SchemaPort[]>
    animated?: boolean
  }

  let {
    edge,
    nodes,
    pending = null,
    outputPortsMap = {},
    inputPortsMap = {},
    animated = false,
  }: Props = $props()

  const CARD_WIDTH = 248
  const HEADER_HEIGHT = 60
  const PORT_SPACING = 28
  const PORT_OFFSET = 20

  function portPoint(
    node: WorkflowGraphNode,
    side: 'left' | 'right',
    portId?: string
  ) {
    let index = 0
    if (portId) {
      const portList =
        side === 'right'
          ? (outputPortsMap[node.type] ?? [])
          : (inputPortsMap[node.type] ?? [])
      const found = portList.findIndex((p) => p.id === portId)
      if (found >= 0) index = found
    }
    return {
      x: node.position.x + (side === 'left' ? 0 : CARD_WIDTH),
      y: node.position.y + HEADER_HEIGHT + PORT_OFFSET + index * PORT_SPACING,
    }
  }

  function findNode(nodeId: string) {
    return nodes.find((node) => node.id === nodeId) ?? null
  }

  const path = $derived.by(() => {
    if (pending) {
      const fromNode = findNode(pending.from_node)
      if (!fromNode) return ''
      const start = portPoint(fromNode, 'right', pending.from_port)
      const end = { x: pending.x, y: pending.y }
      const dx = Math.max(48, Math.abs(end.x - start.x) * 0.45)
      return `M ${start.x} ${start.y} C ${start.x + dx} ${start.y}, ${end.x - dx} ${end.y}, ${end.x} ${end.y}`
    }

    if (!edge) return ''
    const fromNode = findNode(edge.from_node)
    const toNode = findNode(edge.to_node)
    if (!fromNode || !toNode) return ''
    const start = portPoint(fromNode, 'right', edge.from_port)
    const end = portPoint(toNode, 'left', edge.to_port)
    const dx = Math.max(48, Math.abs(end.x - start.x) * 0.45)
    return `M ${start.x} ${start.y} C ${start.x + dx} ${start.y}, ${end.x - dx} ${end.y}, ${end.x} ${end.y}`
  })
</script>

{#if path}
  <path
    d={path}
    fill="none"
    stroke={pending
      ? 'rgba(14, 165, 233, 0.9)'
      : animated
        ? 'rgba(14, 165, 233, 0.75)'
        : 'rgba(100, 116, 139, 0.85)'}
    stroke-width="2.5"
    stroke-dasharray={animated ? '6 4' : undefined}
    stroke-linecap="round"
    class={animated ? 'animated-edge' : ''}
  />
{/if}

<style>
  @keyframes dashdraw {
    to {
      stroke-dashoffset: -10;
    }
  }
  :global(.animated-edge) {
    animation: dashdraw 0.4s linear infinite;
  }
</style>
