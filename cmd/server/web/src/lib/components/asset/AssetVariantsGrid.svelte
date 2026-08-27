<script lang="ts">
  import { type Asset, type Variant } from '$lib/api'
  import VariantCard from '$lib/components/variants/VariantCard.svelte'

  interface Props {
    asset: Asset
    variants: readonly Variant[]
    selectedVariant: Variant | null
    onSelectVariant: (v: Variant) => void
    onVariantUpdated: (variant: Variant) => void
    onVariantsUpdated: (variants: readonly Variant[]) => void
    deleteVariant: (variantId: string) => void
    promoteVariant: (variant: Variant) => void
    thumbnailUpdated: () => void
    rerunVariant: () => void
    reuseVariant: (variant: Variant) => void
  }

  let {
    asset,
    variants,
    selectedVariant,
    onSelectVariant,
    onVariantUpdated,
    onVariantsUpdated,
    deleteVariant,
    promoteVariant,
    thumbnailUpdated,
    rerunVariant,
    reuseVariant,
  }: Props = $props()
</script>

<div class="grid grid-cols-2 gap-3">
  {#each variants as v}
    <VariantCard
      variant={v}
      assetId={asset.id}
      assetMimeType={asset.mime_type}
      isSelected={selectedVariant?.id === v.id}
      onSelect={() => onSelectVariant(v)}
      {onVariantUpdated}
      {onVariantsUpdated}
      onDelete={() => deleteVariant(v.id)}
      onPromote={() => promoteVariant(v)}
      onThumbnailUpdated={thumbnailUpdated}
      onRerun={rerunVariant}
      onReuse={() => reuseVariant(v)}
    />
  {/each}
</div>
