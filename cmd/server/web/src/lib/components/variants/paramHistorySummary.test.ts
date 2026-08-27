import { describe, expect, it } from 'vitest'
import { formatParamSummary } from './paramHistorySummary'

describe('formatParamSummary', () => {
  it('image_with_prompt: shows truncated prompt and short model name', () => {
    const result = formatParamSummary('image_with_prompt', {
      prompt: 'A cat wearing a sombrero in a sunny Mexican plaza',
      model: 'black-forest-labs/FLUX.1-fill-dev',
    })
    expect(result).toContain('A cat wearing')
    expect(result).toContain('FLUX.1-fill-dev')
  })

  it('image_with_prompt: no model segment when model absent', () => {
    const result = formatParamSummary('image_with_prompt', { prompt: 'hello' })
    expect(result).toBe('hello')
  })

  it('image_with_prompt: truncates long prompts at 55 chars', () => {
    const long = 'x'.repeat(80)
    expect(
      formatParamSummary('image_with_prompt', { prompt: long })
    ).toHaveLength(56) // 55 + ellipsis
  })

  it('image_with_prompt: shows leading "#" line as description, not the prompt', () => {
    const result = formatParamSummary('image_with_prompt', {
      prompt: '# vintage filter\nadd a warm vintage film grain',
      model: 'black-forest-labs/FLUX.1-fill-dev',
    })
    expect(result).toBe('vintage filter · FLUX.1-fill-dev')
  })

  it('image_watermark: shows position, opacity percentage, scale percentage', () => {
    const result = formatParamSummary('image_watermark', {
      position: 'bottom-right',
      opacity: 0.8,
      scale: 0.25,
    })
    expect(result).toBe('bottom-right · 80% · 25%')
  })

  it('video_watermark: uses the same formatting as image_watermark', () => {
    const result = formatParamSummary('video_watermark', {
      position: 'center',
      opacity: 0.5,
    })
    expect(result).toBe('center · 50%')
  })

  it('image_resize: shows dimensions and fit', () => {
    expect(
      formatParamSummary('image_resize', {
        width: 1920,
        height: 1080,
        fit: 'cover',
      })
    ).toBe('1920 × 1080 (cover)')
  })

  it('image_resize: omits fit when absent', () => {
    expect(
      formatParamSummary('image_resize', { width: 400, height: 300 })
    ).toBe('400 × 300')
  })

  it('image_convert: uppercases format and shows quality', () => {
    expect(
      formatParamSummary('image_convert', { format: 'webp', quality: 85 })
    ).toBe('WEBP · q85')
  })

  it('image_crop / image_smart_crop: shows aspect ratio', () => {
    expect(formatParamSummary('image_crop', { aspect_ratio: '16:9' })).toBe(
      '16:9'
    )
    expect(
      formatParamSummary('image_smart_crop', { aspect_ratio: '1:1' })
    ).toBe('1:1')
  })

  it('video_transcode: shows format, resolution, bitrate', () => {
    expect(
      formatParamSummary('video_transcode', {
        format: 'mp4',
        resolution: '1080p',
        bitrate: '4000k',
      })
    ).toBe('MP4 · 1080p · 4000k')
  })

  it('audio_transcode: shows format, bitrate, mono flag', () => {
    expect(
      formatParamSummary('audio_transcode', {
        format: 'mp3',
        bitrate: '192k',
        mono: true,
      })
    ).toBe('MP3 · 192k · mono')
  })

  it('audio_normalize: shows LUFS and optional format', () => {
    expect(
      formatParamSummary('audio_normalize', { target_lufs: -16, format: 'mp3' })
    ).toBe('-16 LUFS · MP3')
    expect(
      formatParamSummary('audio_normalize', {
        target_lufs: -23,
        format: 'source',
      })
    ).toBe('-23 LUFS')
  })

  it('custom_ffmpeg: shows truncated command', () => {
    const result = formatParamSummary('custom_ffmpeg', {
      command:
        'ffmpeg -i {input} -vf scale=1280:-2 -c:v libx264 -crf 23 -preset fast {output}',
    })
    expect(result.length).toBeLessThanOrEqual(61) // 60 + ellipsis
    expect(result.startsWith('ffmpeg -i {input}')).toBe(true)
  })

  it('custom_ffmpeg: collapses internal newlines and repeated whitespace', () => {
    const result = formatParamSummary('custom_ffmpeg', {
      command: 'ffmpeg -i {input}\n  -vf scale=640:-2\n  {output}',
    })
    expect(result).toBe('ffmpeg -i {input} -vf scale=640:-2 {output}')
  })

  it('custom_ffmpeg: short command is not truncated', () => {
    const result = formatParamSummary('custom_ffmpeg', {
      command: 'ffmpeg -i {input} -c copy {output}',
    })
    expect(result).toBe('ffmpeg -i {input} -c copy {output}')
  })

  it('custom_ffmpeg: shows leading "#" line as description, not the command', () => {
    const result = formatParamSummary('custom_ffmpeg', {
      command:
        '# downscale for web\nffmpeg -i {input} -vf scale=1280:-2 {output}',
    })
    expect(result).toBe('downscale for web')
  })

  it('unknown type: falls back to JSON dump', () => {
    const result = formatParamSummary('mystery_tool', { foo: 'bar' })
    expect(result).toBe('{"foo":"bar"}')
  })
})
