UPDATE variants SET type = 'image_smart_crop' WHERE type = 'image_smartcrop';
UPDATE variants SET type = 'video_extract'    WHERE type = 'extract_audio';
UPDATE variants SET type = 'audio_transcode'  WHERE type = 'transcode_audio';
UPDATE variants SET type = 'audio_normalize'  WHERE type = 'normalize_audio';

UPDATE jobs SET type = 'image_smart_crop' WHERE type = 'image_smartcrop';
UPDATE jobs SET type = 'video_extract'    WHERE type = 'extract_audio';
UPDATE jobs SET type = 'audio_transcode'  WHERE type = 'transcode_audio';
UPDATE jobs SET type = 'audio_normalize'  WHERE type = 'normalize_audio';
