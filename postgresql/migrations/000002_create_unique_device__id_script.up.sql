ALTER TABLE public.tx_notifications ADD CONSTRAINT tx_notifications_device__id_script UNIQUE (device_id,script);