package ai

// DefaultTranscriptionModel is the OpenRouter model used for audio
// transcription when the user does not specify one.
const DefaultTranscriptionModel = "openai/whisper-1"

// MaxTranscribeAudioBytes caps the source audio size read into memory and
// base64-encoded for the speech-to-text API request. OpenRouter's upstream
// STT provider times out around 60s, so very large files are rejected
// upfront rather than left to fail mid-request.
const MaxTranscribeAudioBytes = 25 * 1024 * 1024

// TranscriptionFormatFromMimeType maps a MIME type to the audio format
// string OpenRouter's speech-to-text endpoint expects in input_audio.format
// (one of: wav, mp3, flac, m4a, ogg, webm, aac). This is intentionally
// separate from transform.AudioFormatFromMimeType, which selects an ffmpeg
// output codec for transcode/normalize jobs and uses a different,
// overlapping-but-distinct vocabulary (e.g. it has no "m4a" or "webm" key,
// and would mis-map an MP4/M4A container to the raw "aac" codec name).
// ok is false when mimeType has no known mapping — callers should skip
// transcription rather than guess and send a request that will fail.
func TranscriptionFormatFromMimeType(mimeType string) (format string, ok bool) {
	switch mimeType {
	case "audio/mpeg", "audio/mp3":
		return "mp3", true
	case "audio/wav", "audio/x-wav", "audio/wave":
		return "wav", true
	case "audio/flac", "audio/x-flac":
		return "flac", true
	case "audio/mp4", "audio/x-m4a":
		return "m4a", true
	case "audio/aac":
		return "aac", true
	case "audio/ogg", "audio/opus":
		return "ogg", true
	case "audio/webm":
		return "webm", true
	default:
		return "", false
	}
}
