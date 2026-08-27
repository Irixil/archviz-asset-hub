/** Maps a variant type + its transform_params to a short human-readable label for the history dropdown. */
export function formatParamSummary(
  type: string,
  params: Record<string, unknown>
): string {
  switch (type) {
    case 'image_with_prompt': {
      const prompt = String(params.prompt ?? '')
      const desc = leadingDescription(prompt)
      const model = params.model ? ` · ${shortModel(String(params.model))}` : ''
      return truncate(desc ?? prompt, 55) + model
    }
    case 'image_watermark':
    case 'video_watermark': {
      const pos = params.position ?? 'center'
      const opacity =
        params.opacity != null
          ? `${Math.round(Number(params.opacity) * 100)}%`
          : ''
      const scale =
        params.scale != null ? `${Math.round(Number(params.scale) * 100)}%` : ''
      return [pos, opacity, scale].filter(Boolean).join(' · ')
    }
    case 'image_resize': {
      const w = params.width ?? '?'
      const h = params.height ?? '?'
      const fit = params.fit ? ` (${params.fit})` : ''
      return `${w} × ${h}${fit}`
    }
    case 'image_convert': {
      const fmt = String(params.format ?? '').toUpperCase()
      const q = params.quality != null ? ` · q${params.quality}` : ''
      return `${fmt}${q}`
    }
    case 'image_smart_crop':
    case 'image_crop': {
      return String(params.aspect_ratio ?? '?')
    }
    case 'video_transcode': {
      const fmt = String(params.format ?? '?').toUpperCase()
      const res = params.resolution ? ` · ${params.resolution}` : ''
      const bitrate = params.bitrate ? ` · ${params.bitrate}` : ''
      return `${fmt}${res}${bitrate}`
    }
    case 'audio_transcode': {
      const fmt = String(params.format ?? '?').toUpperCase()
      const bitrate = params.bitrate ? ` · ${params.bitrate}` : ''
      const mono = params.mono ? ' · mono' : ''
      return `${fmt}${bitrate}${mono}`
    }
    case 'audio_normalize': {
      const lufs =
        params.target_lufs != null ? `${params.target_lufs} LUFS` : '?'
      const fmt =
        params.format && params.format !== 'source'
          ? ` · ${String(params.format).toUpperCase()}`
          : ''
      return `${lufs}${fmt}`
    }
    case 'custom_ffmpeg': {
      const raw = String(params.command ?? '')
      const desc = leadingDescription(raw)
      if (desc) return truncate(desc, 60)
      // Commands run up to 2000 chars — collapse whitespace/newlines so a multi-line
      // command still reads as a single dropdown row, then truncate hard.
      const cmd = raw.replace(/\s+/g, ' ').trim()
      return truncate(cmd, 60)
    }
    default:
      return JSON.stringify(params)
  }
}

function truncate(s: string, n: number): string {
  return s.length > n ? s.slice(0, n) + '…' : s
}

// Extracts the label from a leading "# ..." line, mirroring the backend's
// transform.StripLeadingDescription — the line is a user-authored
// description, not part of the prompt/command sent for processing.
function leadingDescription(text: string): string | null {
  const nl = text.indexOf('\n')
  const firstLine = (nl >= 0 ? text.slice(0, nl) : text).trim()
  return firstLine.startsWith('#') ? firstLine.slice(1).trim() : null
}

// Shorten "black-forest-labs/FLUX.1-fill-dev" → "FLUX.1-fill-dev"
function shortModel(model: string): string {
  const parts = model.split('/')
  return parts[parts.length - 1]
}
