<script lang="ts">
  import { variantApi } from '$lib/api'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    value: string
    disabled?: boolean
    error?: string | null
  }

  let {
    value = $bindable(''),
    disabled = false,
    error = null,
  }: Props = $props()

  const MAX_LEN = 2000
  const TOKEN_PARAMS = { input: '{input}', output: '{output}' }

  let localError = $state<string | null>(null)
  let serverChecking = $state(false)
  let serverResult = $state<{ ok: boolean; message: string } | null>(null)
  let debounceId: ReturnType<typeof setTimeout> | undefined

  function validateLocally(cmd: string): string | null {
    if (!cmd.trim()) return null
    if (cmd.length > MAX_LEN)
      return m.variants_custom_ffmpeg_error_command_too_long()
    if ((cmd.match(/\{input\}/g) ?? []).length !== 1) {
      return m.variants_custom_ffmpeg_error_missing_input_token(TOKEN_PARAMS)
    }
    if ((cmd.match(/\{output\}/g) ?? []).length !== 1) {
      return m.variants_custom_ffmpeg_error_missing_output_token(TOKEN_PARAMS)
    }
    if (/[&|`$<>]/.test(cmd) || /\.\./.test(cmd)) {
      return m.variants_custom_ffmpeg_error_command_blacklisted({ detail: '' })
    }
    return null
  }

  $effect(() => {
    const cmd = value
    clearTimeout(debounceId)
    serverResult = null
    debounceId = setTimeout(() => {
      localError = validateLocally(cmd)
    }, 400)
    return () => clearTimeout(debounceId)
  })

  async function handleValidateOnServer() {
    serverChecking = true
    serverResult = null
    try {
      const res = await variantApi.validateCommand(value)
      if (res.valid) {
        serverResult = {
          ok: true,
          message: m.variants_custom_ffmpeg_validate_success(),
        }
      } else {
        const detail = res.detail ?? ''
        const key = res.error ?? 'command_blacklisted'
        const messages: Record<string, string> = {
          command_required: m.variants_custom_ffmpeg_error_command_required(),
          command_too_long: m.variants_custom_ffmpeg_error_command_too_long(),
          missing_input_token:
            m.variants_custom_ffmpeg_error_missing_input_token(TOKEN_PARAMS),
          missing_output_token:
            m.variants_custom_ffmpeg_error_missing_output_token(TOKEN_PARAMS),
          command_blacklisted:
            m.variants_custom_ffmpeg_error_command_blacklisted({ detail }),
          bad_ref_token: m.variants_custom_ffmpeg_error_bad_ref_token({
            detail,
          }),
        }
        serverResult = {
          ok: false,
          message: messages[key] ?? m.variants_custom_ffmpeg_error_generic(),
        }
      }
    } catch {
      serverResult = {
        ok: false,
        message: m.variants_custom_ffmpeg_error_generic(),
      }
    } finally {
      serverChecking = false
    }
  }
</script>

<div class="ffmpeg-command-input">
  <label for="ffmpeg-command-textarea" class="field-label">
    {m.variants_custom_ffmpeg_command_label()}
  </label>
  <div class="textarea-wrap">
    <textarea
      id="ffmpeg-command-textarea"
      bind:value
      {disabled}
      rows="4"
      class="command-textarea"
      placeholder={m.variants_custom_ffmpeg_command_placeholder(TOKEN_PARAMS)}
      maxlength={MAX_LEN}
      spellcheck="false"
    ></textarea>
    <span class="char-counter">{value.length}/{MAX_LEN}</span>
  </div>

  <p class="command-hint">
    {m.variants_custom_ffmpeg_command_hint(TOKEN_PARAMS)}
  </p>
  <p class="command-hint">
    {m.variants_custom_ffmpeg_refs_hint()}
  </p>

  {#if localError}
    <p class="validation-message error">{localError}</p>
  {/if}
  {#if error}
    <p class="validation-message error">{error}</p>
  {/if}
  {#if serverResult}
    <p
      class="validation-message"
      class:error={!serverResult.ok}
      class:success={serverResult.ok}
    >
      {serverResult.message}
    </p>
  {/if}

  <button
    type="button"
    class="btn-validate"
    disabled={disabled || serverChecking || !value.trim()}
    onclick={handleValidateOnServer}
  >
    {serverChecking
      ? m.variants_custom_ffmpeg_validating()
      : m.variants_custom_ffmpeg_validate_button()}
  </button>
</div>

<style>
  .ffmpeg-command-input {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .field-label {
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-secondary);
  }
  .textarea-wrap {
    position: relative;
  }
  .command-textarea {
    width: 100%;
    min-height: 96px;
    resize: vertical;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    color: var(--text-primary);
    padding: 8px 10px 22px;
    font-size: 0.8125rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    outline: none;
    transition: border-color 0.12s ease;
  }
  .command-textarea:focus {
    border-color: var(--accent-cta);
  }
  .char-counter {
    position: absolute;
    right: 8px;
    bottom: 6px;
    font-size: 0.6875rem;
    color: var(--text-muted);
    pointer-events: none;
  }
  .command-hint {
    font-size: 0.75rem;
    color: var(--text-muted);
  }
  .validation-message {
    font-size: 0.75rem;
    margin: 0;
  }
  .validation-message.error {
    color: oklch(55% 0.2 25);
  }
  .validation-message.success {
    color: oklch(55% 0.16 145);
  }
  .btn-validate {
    align-self: flex-start;
    padding: 5px 12px;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-secondary);
    font-size: 0.8125rem;
    cursor: pointer;
    transition:
      background 120ms ease,
      color 120ms ease;
  }
  .btn-validate:not(:disabled):hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  .btn-validate:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
