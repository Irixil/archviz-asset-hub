<script lang="ts">
  import type {
    WorkflowGraph,
    WorkflowGraphNode,
    WorkflowNodeSchema,
  } from '$lib/api/workflows'
  import EdgeSvg from './EdgeSvg.svelte'
  import NodeCard from './NodeCard.svelte'
  import NodeConfigPanel from './NodeConfigPanel.svelte'
  import NodePalette from './NodePalette.svelte'

  interface StepStatusMap {
    [nodeId: string]: string
  }

  interface StepCtxMap {
    [nodeId: string]: {
      input: Record<string, unknown>
      output: Record<string, unknown>
    }
  }

  interface Props {
    graph?: WorkflowGraph
    schemas: WorkflowNodeSchema[]
    readonly?: boolean
    stepStatuses?: StepStatusMap
    stepCtx?: StepCtxMap
  }

  let {
    graph = $bindable({ nodes: [], edges: [] }),
    schemas,
    readonly = false,
    stepStatuses = {},
    stepCtx = {},
  }: Props = $props()

  let root = $state<HTMLDivElement | null>(null)
  let canvas = $state<HTMLDivElement | null>(null)
  let selectedNodeId = $state<string | null>(null)
  let transform = $state({ x: 0, y: 0, scale: 1 })
  let isPanning = $state(false)
  let pendingEdge = $state<{
    from_node: string
    from_port: string
    x: number
    y: number
  } | null>(null)

  const selectedNode = $derived(
    graph.nodes.find((node) => node.id === selectedNodeId) ?? null
  )

  function schemaFor(nodeType: string) {
    return schemas.find((schema) => schema.type === nodeType) ?? null
  }

  const outputPortsMap = $derived.by(() => {
    const map: Record<string, { id: string; label: string }[]> = {}
    for (const schema of schemas) {
      map[schema.type] = schema.outputs
    }
    return map
  })

  const inputPortsMap = $derived.by(() => {
    const map: Record<string, { id: string; label: string }[]> = {}
    for (const schema of schemas) {
      map[schema.type] = schema.inputs
    }
    return map
  })

  function defaultConfig(nodeType: string) {
    const schema = schemaFor(nodeType)
    const props = schema?.config_schema.properties ?? {}
    const out: Record<string, unknown> = {}
    for (const [key, field] of Object.entries(props)) {
      if (field.type === 'boolean') out[key] = false
    }
    return out
  }

  function addNode(nodeType: string) {
    const nodeSchema = schemaFor(nodeType)
    if (!nodeSchema) return
    if (nodeSchema.category === 'trigger') {
      const alreadyHasTrigger = graph.nodes.some(
        (node) => schemaFor(node.type)?.category === 'trigger'
      )
      if (alreadyHasTrigger) return
    }

    const bounds = canvas?.getBoundingClientRect()
    const viewX = bounds ? bounds.width / 2 : 240
    const viewY = bounds ? bounds.height / 2 : 180
    const x = (viewX - transform.x) / transform.scale - 120
    const y = (viewY - transform.y) / transform.scale - 60
    const node: WorkflowGraphNode = {
      id: `${nodeType.replace(/[^a-z0-9]+/gi, '_')}_${Math.random().toString(36).slice(2, 8)}`,
      type: nodeType,
      config: defaultConfig(nodeType),
      position: { x, y },
    }
    graph = { ...graph, nodes: [...graph.nodes, node] }
    selectedNodeId = node.id
  }

  function updateNode(nodeId: string, patch: Partial<WorkflowGraphNode>) {
    graph = {
      ...graph,
      nodes: graph.nodes.map((node) =>
        node.id === nodeId ? { ...node, ...patch } : node
      ),
    }
  }

  function moveNode(nodeId: string, dx: number, dy: number) {
    const node = graph.nodes.find((item) => item.id === nodeId)
    if (!node) return
    updateNode(nodeId, {
      position: {
        x: node.position.x + dx / transform.scale,
        y: node.position.y + dy / transform.scale,
      },
    })
  }

  function updateNodeConfig(nodeId: string, config: Record<string, unknown>) {
    updateNode(nodeId, { config })
  }

  function deleteNode(nodeId: string) {
    graph = {
      nodes: graph.nodes.filter((node) => node.id !== nodeId),
      edges: graph.edges.filter(
        (edge) => edge.from_node !== nodeId && edge.to_node !== nodeId
      ),
    }
    if (selectedNodeId === nodeId) {
      selectedNodeId = null
    }
  }

  function duplicateNode(nodeId: string) {
    const node = graph.nodes.find((n) => n.id === nodeId)
    if (!node) return
    const newId = `node_${Math.random().toString(36).slice(2, 8)}`
    const newNode = {
      ...node,
      id: newId,
      position: { x: node.position.x + 32, y: node.position.y + 32 },
    }
    graph = { ...graph, nodes: [...graph.nodes, newNode] }
    selectedNodeId = newId
  }

  function handleZoom(event: WheelEvent) {
    if (!event.shiftKey) return
    event.preventDefault()
    const delta = event.deltaY < 0 ? 0.08 : -0.08
    transform = {
      ...transform,
      scale: Math.min(1.8, Math.max(0.45, transform.scale + delta)),
    }
  }

  function startPan(event: PointerEvent) {
    if (event.button !== 0 || !event.shiftKey) return
    isPanning = true
    const originX = event.clientX
    const originY = event.clientY
    const start = { ...transform }
    const target = event.currentTarget as HTMLElement
    target.setPointerCapture(event.pointerId)

    const move = (next: PointerEvent) => {
      transform = {
        ...transform,
        x: start.x + (next.clientX - originX),
        y: start.y + (next.clientY - originY),
      }
    }
    const end = () => {
      isPanning = false
      target.removeEventListener('pointermove', move)
      target.removeEventListener('pointerup', end)
      target.removeEventListener('pointercancel', end)
    }

    target.addEventListener('pointermove', move)
    target.addEventListener('pointerup', end)
    target.addEventListener('pointercancel', end)
  }

  function pointerToGraph(event: PointerEvent) {
    const bounds = canvas?.getBoundingClientRect()
    const localX = event.clientX - (bounds?.left ?? 0)
    const localY = event.clientY - (bounds?.top ?? 0)
    return {
      x: (localX - transform.x) / transform.scale,
      y: (localY - transform.y) / transform.scale,
    }
  }

  function startEdge(nodeId: string, portId: string, event: PointerEvent) {
    if (readonly) return
    const point = pointerToGraph(event)
    pendingEdge = {
      from_node: nodeId,
      from_port: portId,
      x: point.x,
      y: point.y,
    }
    selectedNodeId = nodeId
  }

  function completeEdge(nodeId: string, portId: string) {
    if (!pendingEdge) return
    if (pendingEdge.from_node === nodeId) {
      pendingEdge = null
      return
    }
    const duplicate = graph.edges.some(
      (edge) =>
        edge.from_node === pendingEdge?.from_node &&
        edge.from_port === pendingEdge?.from_port &&
        edge.to_node === nodeId &&
        edge.to_port === portId
    )
    const inputOccupied = graph.edges.some(
      (edge) => edge.to_node === nodeId && edge.to_port === portId
    )
    if (!duplicate && !inputOccupied) {
      graph = {
        ...graph,
        edges: [
          ...graph.edges,
          {
            id: `edge_${Math.random().toString(36).slice(2, 8)}`,
            from_node: pendingEdge.from_node,
            from_port: pendingEdge.from_port,
            to_node: nodeId,
            to_port: portId,
          },
        ],
      }
    }
    pendingEdge = null
  }

  function cancelPendingEdge() {
    pendingEdge = null
  }

  function deleteEdgesForPort(
    nodeId: string,
    portId: string,
    isOutput: boolean
  ) {
    graph = {
      ...graph,
      edges: graph.edges.filter((edge) =>
        isOutput
          ? !(edge.from_node === nodeId && edge.from_port === portId)
          : !(edge.to_node === nodeId && edge.to_port === portId)
      ),
    }
  }

  $effect(() => {
    if (!pendingEdge || typeof window === 'undefined') return

    const move = (event: PointerEvent) => {
      const point = pointerToGraph(event)
      pendingEdge = pendingEdge
        ? { ...pendingEdge, x: point.x, y: point.y }
        : null
    }
    const end = () => cancelPendingEdge()

    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', end)
    return () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', end)
    }
  })
</script>

<div
  bind:this={root}
  class="grid gap-4 {readonly
    ? 'grid-cols-1'
    : 'xl:grid-cols-[280px_minmax(0,1fr)_400px]'}"
>
  {#if !readonly}
    <NodePalette
      {schemas}
      onAdd={addNode}
      hasTrigger={graph.nodes.some(
        (n) => schemaFor(n.type)?.category === 'trigger'
      )}
    />
  {/if}

  <div
    bind:this={canvas}
    role="presentation"
    class="relative min-h-[680px] flex-1 overflow-hidden rounded-[28px] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] {isPanning
      ? 'cursor-grabbing'
      : readonly
        ? ''
        : 'cursor-default'}"
    style="background-image: radial-gradient(circle, rgba(100,116,139,0.25) 1px, transparent 1px); background-size: 24px 24px;"
    onwheel={handleZoom}
    onpointerdown={startPan}
  >
    <div
      class="absolute inset-0 overflow-visible transition-transform"
      style:transform={`translate(${transform.x}px, ${transform.y}px) scale(${transform.scale})`}
    >
      <svg
        class="pointer-events-none absolute inset-0 h-full w-full overflow-visible"
      >
        {#each graph.edges as edge, index (`${edge.id ?? 'edge'}-${index}`)}
          <EdgeSvg
            {edge}
            nodes={graph.nodes}
            {outputPortsMap}
            {inputPortsMap}
            animated={stepStatuses[edge.from_node] === 'completed' &&
              edge.to_node in stepStatuses}
          />
        {/each}
        {#if pendingEdge}
          <EdgeSvg
            nodes={graph.nodes}
            pending={pendingEdge}
            {outputPortsMap}
            {inputPortsMap}
          />
        {/if}
      </svg>

      {#each graph.nodes as node (node.id)}
        <NodeCard
          {node}
          schema={schemaFor(node.type)}
          selected={!readonly && node.id === selectedNodeId}
          {readonly}
          status={stepStatuses[node.id] ?? null}
          inputCtx={stepCtx[node.id]?.input ?? null}
          outputCtx={stepCtx[node.id]?.output ?? null}
          onSelect={() => {
            selectedNodeId = node.id
          }}
          onMove={(dx, dy) => moveNode(node.id, dx, dy)}
          onPortDown={(portId, event) => startEdge(node.id, portId, event)}
          onPortUp={(portId) => completeEdge(node.id, portId)}
          onPortContextMenu={(portId, isOutput) =>
            deleteEdgesForPort(node.id, portId, isOutput)}
        />
      {/each}
    </div>

    {#if !readonly}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        onclick={() => (transform = { ...transform, scale: 1 })}
        class="zoom-level absolute right-4 bottom-4 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3 py-1.5 font-mono text-[11px] font-medium text-[var(--text-muted)] shadow-sm select-none"
      >
        {Math.round(transform.scale * 100)}%{isPanning ? ' · panning' : ''}
      </div>
    {/if}
  </div>

  {#if !readonly}
    <NodeConfigPanel
      node={selectedNode}
      schema={selectedNode ? schemaFor(selectedNode.type) : null}
      onUpdate={(config) => {
        if (selectedNode) updateNodeConfig(selectedNode.id, config)
      }}
      onDelete={() => {
        if (selectedNode) deleteNode(selectedNode.id)
      }}
      onDuplicate={() => {
        if (selectedNode) duplicateNode(selectedNode.id)
      }}
    />
  {/if}
</div>
