import type {
  PublicAsset,
  PublicShare,
  ShareComment,
  SharedVariant,
} from '$lib/api'
import { publicShareApi } from '$lib/api'

const API_BASE = import.meta.env.VITE_API_URL ?? ''

export const TYPES_BACKGROUNDS: Record<string, string> = {
  image: 'bg-amber-300 dark:bg-amber-700',
  video: 'bg-rose-300 dark:bg-rose-700',
  audio: 'bg-emerald-300 dark:bg-emerald-700',
  document: 'bg-blue-200 dark:bg-blue-700',
}

function createPublicViewStore() {
  // ---- Session ----
  let sessionToken = $state<string | null>(null)
  let visitorName = $state('')

  // ---- Gallery ----
  let share = $state<PublicShare | null>(null)
  let assets = $state<PublicAsset[]>([])
  let loadingGallery = $state(false)
  let galleryError = $state('')

  // ---- Review panel ----
  let selectedAsset = $state<PublicAsset | null>(null)
  let selectedVariant = $state<SharedVariant | null>(null)
  let panelOpen = $state(false)
  let comments = $state<ShareComment[]>([])
  let loadingComments = $state(false)

  // ---- Comment form ----
  let commentEmail = $state('')
  let commentBody = $state('')
  let commentBodyError = $state('')
  let postingComment = $state(false)
  let commentPosted = $state(false)

  // ---- Derived ----
  const previewMimeType = $derived(
    selectedVariant?.mime_type ?? selectedAsset?.mime_type ?? ''
  )

  const previewThumbUrl = $derived.by(() => {
    if (!selectedAsset || !share) return null
    if (selectedVariant)
      return variantThumbUrl(share.id, selectedAsset.id, selectedVariant.id)
    return thumbUrl(share.id, selectedAsset.id)
  })

  const previewFileUrl = $derived.by(() => {
    if (!selectedAsset || !share) return null
    if (selectedVariant)
      return variantFileUrl(share.id, selectedAsset.id, selectedVariant.id)
    return fileUrlWithToken(share.id, selectedAsset.id)
  })

  const expiryWarning = $derived.by(() => {
    if (!share?.expires_at) return null
    const diff = new Date(share.expires_at).getTime() - Date.now()
    const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
    if (days < 0) return 'Expired'
    if (days <= 7) return `Expires in ${days} day${days === 1 ? '' : 's'}`
    return null
  })

  // ---- Helpers ----
  function thumbUrl(shareId: string, assetId: string) {
    return `${API_BASE}/shared/${shareId}/assets/${assetId}/thumb`
  }

  function fileUrl(shareId: string, assetId: string) {
    return `${API_BASE}/shared/${shareId}/assets/${assetId}/file`
  }

  function fileUrlWithToken(shareId: string, assetId: string) {
    const base = fileUrl(shareId, assetId)
    return sessionToken
      ? `${base}?token=${encodeURIComponent(sessionToken)}`
      : base
  }

  function variantFileUrl(shareId: string, assetId: string, variantId: string) {
    return `${API_BASE}/shared/${shareId}/assets/${assetId}/variants/${variantId}/file`
  }

  function variantThumbUrl(
    shareId: string,
    assetId: string,
    variantId: string
  ) {
    return `${API_BASE}/shared/${shareId}/assets/${assetId}/variants/${variantId}/thumb`
  }

  // ---- Actions ----
  async function init(shareId: string) {
    sessionToken =
      (await cookieStore.get(`share_token_${shareId}`))?.value ?? null
    visitorName =
      sessionStorage.getItem(`damask_share_visitor_${shareId}`) ?? ''
    commentEmail = sessionStorage.getItem('damask_comment_email') ?? ''
  }

  async function loadGallery(shareId: string, onNeedsPassword: () => void) {
    loadingGallery = true
    galleryError = ''
    try {
      const res = await publicShareApi.getAssets(shareId, sessionToken)
      if (res.status === 401 || res.status === 403) {
        onNeedsPassword()
        return
      }
      if (!res.ok) {
        galleryError = 'This share link is invalid or has expired.'
        return
      }
      const data = await res.json()
      share = data.share
      assets = data.assets ?? []
    } catch {
      galleryError = 'Failed to load gallery. Please check your connection.'
    } finally {
      loadingGallery = false
    }
  }

  async function loadComments(shareId: string, assetId: string) {
    loadingComments = true
    try {
      const res = await publicShareApi.getComments(
        shareId,
        assetId,
        sessionToken
      )
      comments = res.ok
        ? await res
            .json()
            .then(
              (data: { asset_id: string; comments: ShareComment[] }[]) =>
                data.find((d) => d.asset_id === assetId)?.comments ?? []
            )
        : []
    } catch {
      comments = []
    } finally {
      loadingComments = false
    }
  }

  function selectVariant(variant: SharedVariant) {
    selectedVariant = selectedVariant?.id === variant.id ? null : variant
  }

  function openAsset(shareId: string, asset: PublicAsset) {
    selectedAsset = asset
    selectedVariant = null
    panelOpen = true
    commentPosted = false
    comments = []
    if (share?.allow_comments) {
      loadComments(shareId, asset.id)
    }
  }

  function closePanel() {
    panelOpen = false
    selectedAsset = null
    selectedVariant = null
  }

  function navigateAsset(shareId: string, direction: 'prev' | 'next') {
    if (!selectedAsset) return
    const idx = assets.findIndex((a) => a.id === selectedAsset!.id)
    if (direction === 'prev' && idx > 0) openAsset(shareId, assets[idx - 1])
    if (direction === 'next' && idx < assets.length - 1)
      openAsset(shareId, assets[idx + 1])
  }

  async function postComment(shareId: string) {
    commentBodyError = ''
    if (!commentBody.trim()) {
      commentBodyError = 'Message is required'
      return
    }

    postingComment = true
    try {
      const res = await publicShareApi.postComment(
        shareId,
        {
          asset_id: selectedAsset?.id,
          author_name: visitorName.trim(),
          author_email: commentEmail.trim() || undefined,
          body:
            (selectedVariant ? `@${selectedVariant.id} ` : '') +
            commentBody.trim(),
        },
        sessionToken
      )
      if (res.ok) {
        const newComment = await res.json()
        comments = [...comments, newComment]
        sessionStorage.setItem('damask_comment_email', commentEmail.trim())
        commentBody = ''
        commentPosted = true
        setTimeout(() => (commentPosted = false), 3000)
      }
    } catch {
      // silent
    } finally {
      postingComment = false
    }
  }

  async function downloadAll(shareId: string) {
    const res = await publicShareApi.downloadExport(shareId, sessionToken)
    if (!res.ok) return
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${share?.label || 'share'}.zip`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  return {
    // State (getters)
    get sessionToken() {
      return sessionToken
    },
    get share() {
      return share
    },
    get visitorName() {
      return visitorName
    },
    get assets() {
      return assets
    },
    get loadingGallery() {
      return loadingGallery
    },
    get galleryError() {
      return galleryError
    },
    get selectedAsset() {
      return selectedAsset
    },
    get selectedVariant() {
      return selectedVariant
    },
    get previewMimeType() {
      return previewMimeType
    },
    get previewThumbUrl() {
      return previewThumbUrl
    },
    get previewFileUrl() {
      return previewFileUrl
    },
    get panelOpen() {
      return panelOpen
    },
    get comments() {
      return comments
    },
    get loadingComments() {
      return loadingComments
    },
    get commentEmail() {
      return commentEmail
    },
    set commentEmail(v: string) {
      commentEmail = v
    },
    get commentBody() {
      return commentBody
    },
    set commentBody(v: string) {
      commentBody = v
    },
    get commentBodyError() {
      return commentBodyError
    },
    get postingComment() {
      return postingComment
    },
    get commentPosted() {
      return commentPosted
    },
    // Derived
    get expiryWarning() {
      return expiryWarning
    },
    // Helpers
    thumbUrl,
    fileUrl,
    fileUrlWithToken,
    variantFileUrl,
    variantThumbUrl,
    // Actions
    init,
    loadGallery,
    openAsset,
    closePanel,
    navigateAsset,
    selectVariant,
    postComment,
    downloadAll,
  }
}

export const publicViewStore = createPublicViewStore()
