UPDATE variants SET type = 'image_smartcrop' WHERE type = 'image_smart_crop';
UPDATE variants SET type = 'extract_audio'   WHERE type = 'video_extract';
UPDATE variants SET type = 'transcode_audio' WHERE type = 'audio_transcode';
UPDATE variants SET type = 'normalize_audio' WHERE type = 'audio_normalize';

UPDATE jobs SET type = 'image_smartcrop' WHERE type = 'image_smart_crop';
UPDATE jobs SET type = 'extract_audio'   WHERE type = 'video_extract';
UPDATE jobs SET type = 'transcode_audio' WHERE type = 'audio_transcode';
UPDATE jobs SET type = 'normalize_audio' WHERE type = 'audio_normalize';
