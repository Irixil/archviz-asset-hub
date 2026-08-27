<script lang="ts">
  import '../app.css'
  import { configStore } from '$lib/stores/config.svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import Toast from '$lib/components/ui/Toast.svelte'
  import ShortcutHelp from '$lib/components/shortcuts/ShortcutHelp.svelte'
  import ActionSheet from '$lib/components/ActionSheet.svelte'
  import { keymap } from '$lib/shortcuts/store.svelte'
  import { useShortcuts } from '$lib/shortcuts/context'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { themeStore } from '$lib/stores/theme.svelte'
  import type { Snippet } from 'svelte'
  import type { LayoutData } from './$types'

  let {
    children: pageChildren,
    data,
  }: { children: Snippet; data: LayoutData } = $props()

  // Referencing the store ensures the module loads and $effect.root fires on first render.
  keymap.current

  useShortcuts({
    'history.undo': () => {
      undoStore.undo()
    },
    'history.redo': () => {
      undoStore.redo()
    },
  })

  $effect.pre(() => {
    if (data?.user && data?.workspace && data?.role) {
      authStore.login(
        data.user,
        data.workspace,
        data.role,
        data.totalAssetCount ?? 0
      )
    }
    configStore.load()
  })

  $effect(() => {
    themeStore.init()
  })
</script>

{@html `<!--
  THESIS: 建筑设计师首先看到可用素材，不被筛选和后台控件包围。
  OWN-WORLD: 深石墨工具外壳、温和中性素材画布、单一琥珀信号色、紧凑档案标签与高密度 contact sheet。
  STORY: 进入工作区，搜索或选择资料域，快速判断格式与兼容信息，然后取用素材进入制作软件。
  FIRST VIEWPORT: 左侧固定档案导航，顶部单行搜索与上传，内容区以素材网格为最大面积；专业筛选默认收起。
  FORM: 专业三维素材工作台，用户确认方向，Impeccable seed e331aaf5。
  FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
-->`}
<ActionSheet>
  {#snippet children()}
    {@render pageChildren?.()}
    <Toast />
    <ShortcutHelp />
  {/snippet}
</ActionSheet>
