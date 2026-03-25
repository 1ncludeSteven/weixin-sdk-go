// Package api provides WeChat API types and client implementation.
package api

// BaseInfo is common request metadata attached to every CGI request.
type BaseInfo struct {
	ChannelVersion string `json:"channel_version,omitempty"`
}

// UploadMediaType defines media types for CDN upload.
type UploadMediaType int

const (
	UploadMediaTypeImage UploadMediaType = 1 + iota
	UploadMediaTypeVideo
	UploadMediaTypeFile
	UploadMediaTypeVoice
)

// MessageType defines message type (USER or BOT).
type MessageType int

const (
	MessageTypeNone MessageType = iota
	MessageTypeUser
	MessageTypeBot
)

// MessageItemType defines message item type.
type MessageItemType int

const (
	MessageItemTypeNone MessageItemType = iota
	MessageItemTypeText
	MessageItemTypeImage
	MessageItemTypeVoice
	MessageItemTypeFile
	MessageItemTypeVideo
)

// MessageState defines message state.
type MessageState int

const (
	MessageStateNew MessageState = iota
	MessageStateGenerating
	MessageStateFinish
)

// TypingStatus defines typing indicator status.
type TypingStatus int

const (
	TypingStatusTyping TypingStatus = 1 + iota
	TypingStatusCancel
)

// TextItem represents text content.
type TextItem struct {
	Text string `json:"text,omitempty"`
}

// CDNMedia represents CDN media reference.
type CDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AESKey            string `json:"aes_key,omitempty"`
	EncryptType       int    `json:"encrypt_type,omitempty"`
}

// ImageItem represents image content.
type ImageItem struct {
	Media      *CDNMedia `json:"media,omitempty"`
	ThumbMedia *CDNMedia `json:"thumb_media,omitempty"`
	AESKey     string    `json:"aeskey,omitempty"`
	URL        string    `json:"url,omitempty"`
	MidSize    int       `json:"mid_size,omitempty"`
	ThumbSize  int       `json:"thumb_size,omitempty"`
	ThumbHeight int      `json:"thumb_height,omitempty"`
	ThumbWidth  int      `json:"thumb_width,omitempty"`
	HDSize     int       `json:"hd_size,omitempty"`
}

// VoiceItem represents voice content.
type VoiceItem struct {
	Media         *CDNMedia `json:"media,omitempty"`
	EncodeType    int       `json:"encode_type,omitempty"`
	BitsPerSample int       `json:"bits_per_sample,omitempty"`
	SampleRate    int       `json:"sample_rate,omitempty"`
	Playtime      int       `json:"playtime,omitempty"`
	Text          string    `json:"text,omitempty"`
}

// FileItem represents file attachment.
type FileItem struct {
	Media    *CDNMedia `json:"media,omitempty"`
	FileName string    `json:"file_name,omitempty"`
	MD5      string    `json:"md5,omitempty"`
	Len      string    `json:"len,omitempty"`
}

// VideoItem represents video content.
type VideoItem struct {
	Media      *CDNMedia `json:"media,omitempty"`
	VideoSize  int       `json:"video_size,omitempty"`
	PlayLength int       `json:"play_length,omitempty"`
	VideoMD5   string    `json:"video_md5,omitempty"`
	ThumbMedia *CDNMedia `json:"thumb_media,omitempty"`
	ThumbSize  int       `json:"thumb_size,omitempty"`
	ThumbHeight int      `json:"thumb_height,omitempty"`
	ThumbWidth  int      `json:"thumb_width,omitempty"`
}

// RefMessage represents a referenced (quoted) message.
type RefMessage struct {
	MessageItem *MessageItem `json:"message_item,omitempty"`
	Title       string       `json:"title,omitempty"`
}

// MessageItem represents a single message item.
type MessageItem struct {
	Type         int          `json:"type,omitempty"`
	CreateTimeMs int64        `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64        `json:"update_time_ms,omitempty"`
	IsCompleted  bool         `json:"is_completed,omitempty"`
	MsgID        string       `json:"msg_id,omitempty"`
	RefMsg       *RefMessage  `json:"ref_msg,omitempty"`
	TextItem     *TextItem    `json:"text_item,omitempty"`
	ImageItem    *ImageItem   `json:"image_item,omitempty"`
	VoiceItem    *VoiceItem   `json:"voice_item,omitempty"`
	FileItem     *FileItem    `json:"file_item,omitempty"`
	VideoItem    *VideoItem   `json:"video_item,omitempty"`
}

// WeixinMessage represents a unified WeChat message.
type WeixinMessage struct {
	Seq          int            `json:"seq,omitempty"`
	MessageID    int64          `json:"message_id,omitempty"`
	FromUserID   string         `json:"from_user_id,omitempty"`
	ToUserID     string         `json:"to_user_id,omitempty"`
	ClientID     string         `json:"client_id,omitempty"`
	CreateTimeMs int64          `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64          `json:"update_time_ms,omitempty"`
	DeleteTimeMs int64          `json:"delete_time_ms,omitempty"`
	SessionID    string         `json:"session_id,omitempty"`
	GroupID      string         `json:"group_id,omitempty"`
	MessageType  int            `json:"message_type,omitempty"`
	MessageState int            `json:"message_state,omitempty"`
	ItemList     []*MessageItem `json:"item_list,omitempty"`
	ContextToken string         `json:"context_token,omitempty"`
}

// GetUpdatesReq represents getUpdates request.
type GetUpdatesReq struct {
	SyncBuf       string    `json:"sync_buf,omitempty"`        // deprecated
	GetUpdatesBuf string    `json:"get_updates_buf,omitempty"`
	BaseInfo      *BaseInfo `json:"base_info,omitempty"`
}

// GetUpdatesResp represents getUpdates response.
type GetUpdatesResp struct {
	Ret                int             `json:"ret,omitempty"`
	ErrCode            int             `json:"errcode,omitempty"`
	ErrMsg             string          `json:"errmsg,omitempty"`
	Msgs               []*WeixinMessage `json:"msgs,omitempty"`
	SyncBuf            string          `json:"sync_buf,omitempty"` // deprecated
	GetUpdatesBuf      string          `json:"get_updates_buf,omitempty"`
	LongPollingTimeout int             `json:"longpolling_timeout_ms,omitempty"`
}

// SendMessageReq represents sendMessage request.
type SendMessageReq struct {
	Msg *WeixinMessage `json:"msg,omitempty"`
}

// GetUploadUrlReq represents getUploadUrl request.
type GetUploadUrlReq struct {
	FileKey         string    `json:"filekey,omitempty"`
	MediaType       int       `json:"media_type,omitempty"`
	ToUserID        string    `json:"to_user_id,omitempty"`
	RawSize         int       `json:"rawsize,omitempty"`
	RawFileMD5      string    `json:"rawfilemd5,omitempty"`
	FileSize        int       `json:"filesize,omitempty"`
	ThumbRawSize    int       `json:"thumb_rawsize,omitempty"`
	ThumbRawFileMD5 string    `json:"thumb_rawfilemd5,omitempty"`
	ThumbFileSize   int       `json:"thumb_filesize,omitempty"`
	NoNeedThumb     bool      `json:"no_need_thumb,omitempty"`
	AESKey          string    `json:"aeskey,omitempty"`
	BaseInfo        *BaseInfo `json:"base_info,omitempty"`
}

// GetUploadUrlResp represents getUploadUrl response.
type GetUploadUrlResp struct {
	UploadParam     string `json:"upload_param,omitempty"`
	ThumbUploadParam string `json:"thumb_upload_param,omitempty"`
}

// GetConfigReq represents getConfig request.
type GetConfigReq struct {
	ILinkUserID   string    `json:"ilink_user_id,omitempty"`
	ContextToken  string    `json:"context_token,omitempty"`
	BaseInfo      *BaseInfo `json:"base_info,omitempty"`
}

// GetConfigResp represents getConfig response.
type GetConfigResp struct {
	Ret          int    `json:"ret,omitempty"`
	ErrMsg       string `json:"errmsg,omitempty"`
	TypingTicket string `json:"typing_ticket,omitempty"`
}

// SendTypingReq represents sendTyping request.
type SendTypingReq struct {
	ILinkUserID   string `json:"ilink_user_id,omitempty"`
	TypingTicket  string `json:"typing_ticket,omitempty"`
	Status        int    `json:"status,omitempty"`
}

// QRCodeResponse represents QR code generation response.
type QRCodeResponse struct {
	QRCode           string `json:"qrcode,omitempty"`
	QRCodeImgContent string `json:"qrcode_img_content,omitempty"`
}

// StatusResponse represents QR code status response.
type StatusResponse struct {
	Status      string `json:"status,omitempty"`
	BotToken    string `json:"bot_token,omitempty"`
	ILinkBotID  string `json:"ilink_bot_id,omitempty"`
	BaseURL     string `json:"baseurl,omitempty"`
	ILinkUserID string `json:"ilink_user_id,omitempty"`
}
