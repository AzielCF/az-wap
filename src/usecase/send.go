package usecase

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	globalConfig "github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/domains/app"
	domainChatStorage "github.com/AzielCF/az-wap/domains/chatstorage"
	domainSend "github.com/AzielCF/az-wap/domains/send"
	infraChatStorage "github.com/AzielCF/az-wap/infrastructure/chatstorage"
	pkgError "github.com/AzielCF/az-wap/pkg/error"
	pkgUtils "github.com/AzielCF/az-wap/pkg/utils"
	"github.com/AzielCF/az-wap/ui/rest/helpers"
	"github.com/AzielCF/az-wap/validations"
	"github.com/AzielCF/az-wap/workspace"
	wsDomainChannel "github.com/AzielCF/az-wap/workspace/domain/channel"
	wsDomainCommon "github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/disintegration/imaging"
	fiberUtils "github.com/gofiber/fiber/v2/utils"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type serviceSend struct {
	appService       app.IAppUsecase
	workspaceManager *workspace.Manager
}

func NewSendService(appService app.IAppUsecase, workspaceManager *workspace.Manager) domainSend.ISendUsecase {
	return &serviceSend{
		appService:       appService,
		workspaceManager: workspaceManager,
	}
}

func (service serviceSend) ensureClientForToken(ctx context.Context, token string) error {
	if token == "" || service.appService == nil {
		return nil
	}
	_, err := service.appService.FirstDevice(ctx, token)
	return err
}

func (service serviceSend) getChatStorageForToken(ctx context.Context, token string) (domainChatStorage.IChatStorageRepository, error) {
	adapter, ok := service.workspaceManager.GetAdapter(token)
	if !ok {
		return nil, fmt.Errorf("channel adapter not found for token: %s", token)
	}

	instanceRepo, err := infraChatStorage.GetOrInitInstanceRepository(adapter.ID())
	if err != nil {
		return nil, fmt.Errorf("failed to get channel chatstorage repo: %w", err)
	}

	return instanceRepo, nil
}

// getAdapterForToken returns the ChannelAdapter for the given channelID (token)
func (service serviceSend) getAdapterForToken(ctx context.Context, token string) (wsDomainChannel.ChannelAdapter, error) {
	adapter, ok := service.workspaceManager.GetAdapter(token)
	if !ok {
		return nil, fmt.Errorf("channel adapter not found for ID: %s", token)
	}
	return adapter, nil
}

// wrapSendMessage wraps the message sending process with message ID saving
func (service serviceSend) wrapSendMessage(ctx context.Context, recipient, text, token, quoteID string) (wsDomainCommon.SendResponse, error) {
	logrus.WithFields(logrus.Fields{
		"recipient": recipient,
		"token":     token,
	}).Debug("[SEND] wrapSendMessage called")

	adapter, err := service.getAdapterForToken(ctx, token)
	if err != nil {
		logrus.WithError(err).Error("[SEND] no adapter available for channel")
		return wsDomainCommon.SendResponse{}, err
	}

	resp, err := adapter.SendMessage(ctx, recipient, text, quoteID)
	if err != nil {
		logrus.WithError(err).WithField("recipient", recipient).Error("[SEND] Failed to send message")
		return wsDomainCommon.SendResponse{}, err
	}

	logrus.WithFields(logrus.Fields{
		"message_id": resp.MessageID,
		"recipient":  recipient,
	}).Info("[SEND] Message sent successfully")

	// Store the sent message using chatstorage
	repo, err := service.getChatStorageForToken(ctx, token)
	if err != nil {
		logrus.WithError(err).Warn("[SEND] skipping message storage as no repo found")
		return resp, nil
	}

	// Store message asynchronously
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("[SEND] Recovered from panic in asynchronous message storage: %v", r)
			}
		}()
		storeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		senderJID := adapter.ID() // Best effort as agnostic ID
		if err := repo.StoreSentMessageWithContext(storeCtx, resp.MessageID, senderJID, recipient, text, resp.Timestamp); err != nil {
			logrus.Warnf("Failed to store sent message: %v", err)
		}
	}()

	// Mark Read logic (Agnostic version)
	go func() {
		_ = adapter.SendPresence(context.Background(), recipient, false, false)
	}()

	return resp, nil
}

func (service serviceSend) SendText(ctx context.Context, request domainSend.MessageRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendMessage(ctx, request)
	if err != nil {
		return response, err
	}

	_, err = service.getAdapterForToken(ctx, request.BaseRequest.Token)
	if err != nil {
		return response, err
	}

	recipient := request.BaseRequest.Phone
	if !strings.Contains(recipient, "@") {
		recipient = recipient + "@s.whatsapp.net"
	}

	quoteID := ""
	if request.ReplyMessageID != nil {
		quoteID = *request.ReplyMessageID
	}

	// NOTE: Mentions and Ephemeral Expiration are currently not supported in the agnostic adapter interface.
	// They are dropped for now to ensure compilation and architectural decoupling.
	// Future: Extend ChannelAdapter.SendMessage to accept options/metadata.

	ts, err := service.wrapSendMessage(ctx, recipient, request.Message, request.BaseRequest.Token, quoteID)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.MessageID
	response.Status = fmt.Sprintf("Message sent to %s (server timestamp: %s)", recipient, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendImage(ctx context.Context, request domainSend.ImageRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendImage(ctx, request)
	if err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.BaseRequest.Token)
	if err != nil {
		return response, err
	}

	recipient := request.Phone
	if !strings.Contains(recipient, "@") {
		recipient = recipient + "@s.whatsapp.net"
	}

	var (
		imagePath      string
		imageThumbnail string
		imageName      string
		deletedItems   []string
		oriImagePath   string
	)

	// Ensure temporary files are cleaned up
	defer func() {
		if len(deletedItems) > 0 {
			go pkgUtils.RemoveFile(1, deletedItems...)
		}
	}()

	if request.ImageURL != nil && *request.ImageURL != "" {
		// Download image from URL
		imageData, fileName, err := pkgUtils.DownloadImageFromURL(*request.ImageURL)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to download image from URL %v", err))
		}

		// Check if the downloaded image is WebP and convert to PNG if needed
		mimeType := http.DetectContentType(imageData)
		if mimeType == "image/webp" {
			// Convert WebP to PNG
			webpImage, err := imaging.Decode(bytes.NewReader(imageData))
			if err != nil {
				return response, pkgError.InternalServerError(fmt.Sprintf("failed to decode WebP image %v", err))
			}

			// Change file extension to PNG
			if strings.HasSuffix(strings.ToLower(fileName), ".webp") {
				fileName = fileName[:len(fileName)-5] + ".png"
			} else {
				fileName = fileName + ".png"
			}

			// Convert to PNG format
			var pngBuffer bytes.Buffer
			err = imaging.Encode(&pngBuffer, webpImage, imaging.PNG)
			if err != nil {
				return response, pkgError.InternalServerError(fmt.Sprintf("failed to convert WebP to PNG %v", err))
			}
			imageData = pngBuffer.Bytes()
		}

		oriImagePath = fmt.Sprintf("%s/%s", globalConfig.PathSendItems, fileName)
		imageName = fileName
		err = os.WriteFile(oriImagePath, imageData, 0644)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to save downloaded image %v", err))
		}
	} else if request.Image != nil {
		// Save image to server
		oriImagePath = fmt.Sprintf("%s/%s", globalConfig.PathSendItems, request.Image.Filename)
		err = fasthttp.SaveMultipartFile(request.Image, oriImagePath)
		if err != nil {
			return response, err
		}
		imageName = request.Image.Filename
	}
	deletedItems = append(deletedItems, oriImagePath)

	/* Generate thumbnail with smalled image size */
	srcImage, err := imaging.Open(oriImagePath)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("Failed to open image file '%s' for thumbnail generation: %v. Possible causes: file not found, unsupported format, or permission denied.", oriImagePath, err))
	}

	// Resize Thumbnail
	resizedImage := imaging.Resize(srcImage, 100, 0, imaging.Lanczos)
	imageThumbnail = fmt.Sprintf("%s/thumbnails-%s", globalConfig.PathSendItems, imageName)
	if err = imaging.Save(resizedImage, imageThumbnail); err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to save thumbnail %v", err))
	}
	deletedItems = append(deletedItems, imageThumbnail)

	if request.Compress {
		// Resize image
		openImageBuffer, err := imaging.Open(oriImagePath)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("Failed to open image file '%s' for compression: %v. Possible causes: file not found, unsupported format, or permission denied.", oriImagePath, err))
		}
		newImage := imaging.Resize(openImageBuffer, 600, 0, imaging.Lanczos)
		newImagePath := fmt.Sprintf("%s/new-%s", globalConfig.PathSendItems, imageName)
		if err = imaging.Save(newImage, newImagePath); err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to save image %v", err))
		}
		deletedItems = append(deletedItems, newImagePath)
		imagePath = newImagePath
	} else {
		imagePath = oriImagePath
	}

	// Send to WA server
	dataWaCaption := request.Caption
	dataWaImage, err := os.ReadFile(imagePath)
	if err != nil {
		return response, err
	}

	mediaReq := wsDomainCommon.MediaUpload{
		Caption:  dataWaCaption,
		FileName: imageName,
		Data:     dataWaImage,
		MimeType: http.DetectContentType(dataWaImage),
		ViewOnce: request.ViewOnce,
		Type:     wsDomainCommon.MediaTypeImage,
	}

	quoteID := "" // ImageRequest reply ID not supported yet

	resp, err := adapter.SendMedia(ctx, recipient, mediaReq, quoteID)
	if err != nil {
		return response, err
	}

	// Store sent message (thumbnail/metadata not stored for now in this version, simplified)
	repo, err := service.getChatStorageForToken(ctx, request.BaseRequest.Token)
	if err == nil {
		go func() {
			_ = repo.StoreSentMessageWithContext(context.Background(), resp.MessageID, adapter.ID(), recipient, "🖼️ Image: "+dataWaCaption, resp.Timestamp)
		}()
	}

	response.MessageID = resp.MessageID
	response.Status = fmt.Sprintf("Message sent to %s", request.BaseRequest.Phone)
	return response, nil
}

func (service serviceSend) SendFile(ctx context.Context, request domainSend.FileRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendFile(ctx, request)
	if err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.BaseRequest.Token)
	if err != nil {
		return response, err
	}

	recipient := request.BaseRequest.Phone
	if !strings.Contains(recipient, "@") {
		recipient = recipient + "@s.whatsapp.net"
	}

	fileBytes := helpers.MultipartFormFileHeaderToBytes(request.File)
	fileMimeType := resolveDocumentMIME(request.File.Filename, fileBytes)

	mediaReq := wsDomainCommon.MediaUpload{
		Caption:  request.Caption,
		FileName: request.File.Filename,
		Data:     fileBytes,
		MimeType: fileMimeType,
		Type:     wsDomainCommon.MediaTypeDocument,
	}

	quoteID := ""

	resp, err := adapter.SendMedia(ctx, recipient, mediaReq, quoteID)
	if err != nil {
		return response, err
	}

	// Store sent message
	repo, err := service.getChatStorageForToken(ctx, request.BaseRequest.Token)
	if err == nil {
		go func() {
			_ = repo.StoreSentMessageWithContext(context.Background(), resp.MessageID, adapter.ID(), recipient, "📄 Document: "+request.File.Filename, resp.Timestamp)
		}()
	}

	response.MessageID = resp.MessageID
	response.Status = fmt.Sprintf("Document sent to %s", request.BaseRequest.Phone)
	return response, nil
}

func resolveDocumentMIME(filename string, fileBytes []byte) string {
	extension := strings.ToLower(filepath.Ext(filename))
	if extension != "" {
		if mimeType, ok := pkgUtils.KnownDocumentMIMEByExtension(extension); ok {
			return mimeType
		}

		if mimeType := mime.TypeByExtension(extension); mimeType != "" {
			// Normalizamos algunos MIME comunes a valores esperados por los tests
			if extension == ".zip" {
				// En Windows suele devolverse "application/x-zip-compressed",
				// pero queremos tratarlo siempre como "application/zip".
				if strings.HasPrefix(mimeType, "application/x-zip-") {
					return "application/zip"
				}
			}
			return mimeType
		}
	}

	return http.DetectContentType(fileBytes)
}

func (service serviceSend) SendVideo(ctx context.Context, request domainSend.VideoRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendVideo(ctx, request)
	if err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.BaseRequest.Token)
	if err != nil {
		return response, err
	}

	recipient := request.BaseRequest.Phone
	if !strings.Contains(recipient, "@") {
		recipient = recipient + "@s.whatsapp.net"
	}

	var (
		videoPath    string
		deletedItems []string
	)

	// Ensure temporary files are always removed, even on early returns
	defer func() {
		if len(deletedItems) > 0 {
			// Run cleanup in background with slight delay to avoid race with open handles
			go pkgUtils.RemoveFile(1, deletedItems...)
		}
	}()

	generateUUID := fiberUtils.UUIDv4()

	var oriVideoPath string

	// Determine source of video (URL or uploaded file)
	if request.VideoURL != nil && *request.VideoURL != "" {
		// Download video bytes
		videoBytes, fileName, errDownload := pkgUtils.DownloadVideoFromURL(*request.VideoURL)
		if errDownload != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to download video from URL %v", errDownload))
		}
		// Build file path to save the downloaded video temporarily
		oriVideoPath = fmt.Sprintf("%s/%s", globalConfig.PathSendItems, generateUUID+fileName)
		if errWrite := os.WriteFile(oriVideoPath, videoBytes, 0644); errWrite != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to store downloaded video in server %v", errWrite))
		}
	} else if request.Video != nil {
		// Save uploaded video to server
		oriVideoPath = fmt.Sprintf("%s/%s", globalConfig.PathSendItems, generateUUID+request.Video.Filename)
		err = fasthttp.SaveMultipartFile(request.Video, oriVideoPath)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to store video in server %v", err))
		}
	} else {
		// This should not happen due to validation, but guard anyway
		return response, pkgError.ValidationError("either Video or VideoURL must be provided")
	}

	// Check if ffmpeg is installed
	_, err = exec.LookPath("ffmpeg")
	if err != nil {
		return response, pkgError.InternalServerError("ffmpeg not installed")
	}

	// Generate thumbnail using ffmpeg
	thumbnailVideoPath := fmt.Sprintf("%s/%s", globalConfig.PathSendItems, generateUUID+".png")
	cmdThumbnail := exec.Command("ffmpeg", "-i", oriVideoPath, "-ss", "00:00:01.000", "-vframes", "1", thumbnailVideoPath)
	err = cmdThumbnail.Run()
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to create thumbnail %v", err))
	}

	// Resize Thumbnail
	srcImage, err := imaging.Open(thumbnailVideoPath)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("Failed to open generated video thumbnail image '%s': %v. Possible causes: file not found, unsupported format, or permission denied.", thumbnailVideoPath, err))
	}
	resizedImage := imaging.Resize(srcImage, 100, 0, imaging.Lanczos)
	thumbnailResizeVideoPath := fmt.Sprintf("%s/thumbnails-%s", globalConfig.PathSendItems, generateUUID+".png")
	if err = imaging.Save(resizedImage, thumbnailResizeVideoPath); err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to save thumbnail %v", err))
	}

	deletedItems = append(deletedItems, thumbnailVideoPath)
	deletedItems = append(deletedItems, thumbnailResizeVideoPath)

	// Compress if requested
	if request.Compress {
		compresVideoPath := fmt.Sprintf("%s/%s", globalConfig.PathSendItems, generateUUID+".mp4")

		// Use proper compression settings to reduce file size
		cmdCompress := exec.Command("ffmpeg", "-i", oriVideoPath,
			"-c:v", "libx264",
			"-crf", "28",
			"-preset", "fast",
			"-vf", "scale=720:-2",
			"-c:a", "aac",
			"-b:a", "128k",
			"-movflags", "+faststart",
			"-y", // Overwrite output file if it exists
			compresVideoPath)

		// Capture both stdout and stderr for better error reporting
		output, err := cmdCompress.CombinedOutput()
		if err != nil {
			logrus.Errorf("ffmpeg compression failed: %v, output: %s", err, string(output))
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to compress video: %v", err))
		}

		videoPath = compresVideoPath
		deletedItems = append(deletedItems, compresVideoPath)
	} else {
		videoPath = oriVideoPath
	}
	deletedItems = append(deletedItems, oriVideoPath)

	//Send to WA server
	dataWaVideo, err := os.ReadFile(videoPath)
	if err != nil {
		return response, err
	}

	mediaReq := wsDomainCommon.MediaUpload{
		Caption:  request.Caption,
		FileName: filepath.Base(videoPath),
		Data:     dataWaVideo,
		MimeType: "video/mp4",
		ViewOnce: request.ViewOnce,
		Type:     wsDomainCommon.MediaTypeVideo,
	}

	resp, err := adapter.SendMedia(ctx, recipient, mediaReq, "")
	if err != nil {
		return response, err
	}

	// Store sent message
	repo, err := service.getChatStorageForToken(ctx, request.BaseRequest.Token)
	if err == nil {
		go func() {
			_ = repo.StoreSentMessageWithContext(context.Background(), resp.MessageID, adapter.ID(), recipient, "🎥 Video: "+request.Caption, resp.Timestamp)
		}()
	}

	response.MessageID = resp.MessageID
	response.Status = fmt.Sprintf("Video sent to %s", request.BaseRequest.Phone)
	return response, nil
}

func (service serviceSend) SendContact(ctx context.Context, request domainSend.ContactRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendContact(ctx, request)
	if err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.BaseRequest.Token)
	if err != nil {
		return response, err
	}

	recipient := request.BaseRequest.Phone
	if !strings.Contains(recipient, "@") {
		recipient = recipient + "@s.whatsapp.net"
	}

	resp, err := adapter.SendContact(ctx, recipient, request.ContactName, request.ContactPhone, "")
	if err != nil {
		return response, err
	}

	// Store sent message
	repo, err := service.getChatStorageForToken(ctx, request.BaseRequest.Token)
	if err == nil {
		go func() {
			_ = repo.StoreSentMessageWithContext(context.Background(), resp.MessageID, adapter.ID(), recipient, "👤 "+request.ContactName, resp.Timestamp)
		}()
	}

	response.MessageID = resp.MessageID
	response.Status = fmt.Sprintf("Contact sent to %s", request.BaseRequest.Phone)
	return response, nil
}

func (service serviceSend) SendLink(ctx context.Context, request domainSend.LinkRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendLink(ctx, request)
	if err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.BaseRequest.Token)
	if err != nil {
		return response, err
	}

	recipient := request.BaseRequest.Phone
	if !strings.Contains(recipient, "@") {
		recipient = recipient + "@s.whatsapp.net"
	}

	metadata, err := pkgUtils.GetMetaDataFromURL(request.Link)
	if err != nil {
		logrus.Warnf("Failed to get metadata for link: %v", err)
		// Continue even with error metadata
		metadata = pkgUtils.Metadata{}
	}

	resp, err := adapter.SendLink(ctx, recipient, request.Link, request.Caption, metadata.Title, metadata.Description, metadata.ImageThumb, "")
	if err != nil {
		return response, err
	}

	// Store sent message
	repo, err := service.getChatStorageForToken(ctx, request.BaseRequest.Token)
	if err == nil {
		go func() {
			content := "🔗 " + request.Link
			if request.Caption != "" {
				content = "🔗 " + request.Caption
			}
			_ = repo.StoreSentMessageWithContext(context.Background(), resp.MessageID, adapter.ID(), recipient, content, resp.Timestamp)
		}()
	}

	response.MessageID = resp.MessageID
	response.Status = fmt.Sprintf("Link sent to %s", request.BaseRequest.Phone)
	return response, nil
}

func (service serviceSend) SendLocation(ctx context.Context, request domainSend.LocationRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendLocation(ctx, request)
	if err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.BaseRequest.Token)
	if err != nil {
		return response, err
	}

	recipient := request.BaseRequest.Phone
	if !strings.Contains(recipient, "@") {
		recipient = recipient + "@s.whatsapp.net"
	}

	lat := pkgUtils.StrToFloat64(request.Latitude)
	long := pkgUtils.StrToFloat64(request.Longitude)

	resp, err := adapter.SendLocation(ctx, recipient, lat, long, request.Address, "")
	if err != nil {
		return response, err
	}

	// Store sent message
	repo, err := service.getChatStorageForToken(ctx, request.BaseRequest.Token)
	if err == nil {
		go func() {
			content := "📍 " + request.Latitude + ", " + request.Longitude
			_ = repo.StoreSentMessageWithContext(context.Background(), resp.MessageID, adapter.ID(), recipient, content, resp.Timestamp)
		}()
	}

	response.MessageID = resp.MessageID
	response.Status = fmt.Sprintf("Location sent to %s", request.BaseRequest.Phone)
	return response, nil
}

func (service serviceSend) SendAudio(ctx context.Context, request domainSend.AudioRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendAudio(ctx, request)
	if err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.BaseRequest.Token)
	if err != nil {
		return response, err
	}

	recipient := request.BaseRequest.Phone
	if !strings.Contains(recipient, "@") {
		recipient = recipient + "@s.whatsapp.net"
	}

	var (
		audioBytes    []byte
		audioMimeType string
		fileName      string = "audio.ogg"
	)

	// Handle audio from URL or file
	if request.AudioURL != nil && *request.AudioURL != "" {
		audioBytes, fileName, err = pkgUtils.DownloadAudioFromURL(*request.AudioURL)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to download audio from URL %v", err))
		}
		audioMimeType = http.DetectContentType(audioBytes)
	} else if request.Audio != nil {
		audioBytes = helpers.MultipartFormFileHeaderToBytes(request.Audio)
		audioMimeType = http.DetectContentType(audioBytes)
		fileName = request.Audio.Filename
	}

	if !strings.HasPrefix(strings.ToLower(audioMimeType), "audio/ogg") {
		if _, errFF := exec.LookPath("ffmpeg"); errFF == nil {
			id := fiberUtils.UUIDv4()
			inputPath := fmt.Sprintf("%s/%s-audio-input", globalConfig.PathSendItems, id)
			outputPath := fmt.Sprintf("%s/%s-audio-output.ogg", globalConfig.PathSendItems, id)
			if errWrite := os.WriteFile(inputPath, audioBytes, 0644); errWrite == nil {
				cmd := exec.Command("ffmpeg", "-y", "-i", inputPath, "-acodec", "libopus", "-b:a", "32k", outputPath)
				if out, errRun := cmd.CombinedOutput(); errRun != nil {
					logrus.WithError(errRun).WithField("output", string(out)).Error("failed to transcode audio to ogg")
				} else {
					if newBytes, errRead := os.ReadFile(outputPath); errRead == nil && len(newBytes) > 0 {
						audioBytes = newBytes
						audioMimeType = "audio/ogg; codecs=opus"
						fileName = id + ".ogg"
					}
				}
			}
			go pkgUtils.RemoveFile(0, inputPath, outputPath)
		}
	}

	mediaUpload := wsDomainCommon.MediaUpload{
		Data:     audioBytes,
		FileName: fileName,
		MimeType: audioMimeType,
		Type:     wsDomainCommon.MediaTypeAudio,
		Caption:  "",
		PTT:      true,
	}

	resp, err := adapter.SendMedia(ctx, recipient, mediaUpload, "")
	if err != nil {
		return response, err
	}

	// Store sent message
	repo, err := service.getChatStorageForToken(ctx, request.BaseRequest.Token)
	if err == nil {
		go func() {
			// Save media file for history if needed, or just content marker
			content := "🎤 Audio Message"
			_ = repo.StoreSentMessageWithContext(context.Background(), resp.MessageID, adapter.ID(), recipient, content, resp.Timestamp)
		}()
	}

	response.MessageID = resp.MessageID
	response.Status = fmt.Sprintf("Audio sent to %s", request.BaseRequest.Phone)
	return response, nil
}

func (service serviceSend) SendPoll(ctx context.Context, request domainSend.PollRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendPoll(ctx, request)
	if err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.BaseRequest.Token)
	if err != nil {
		return response, err
	}

	recipient := request.BaseRequest.Phone
	if !strings.Contains(recipient, "@") {
		recipient = recipient + "@s.whatsapp.net"
	}

	resp, err := adapter.SendPoll(ctx, recipient, request.Question, request.Options, request.MaxAnswer, "")
	if err != nil {
		return response, err
	}

	// Store sent message (simplified content for poll)
	repo, err := service.getChatStorageForToken(ctx, request.BaseRequest.Token)
	if err == nil {
		go func() {
			content := "📊 " + request.Question
			_ = repo.StoreSentMessageWithContext(context.Background(), resp.MessageID, adapter.ID(), recipient, content, resp.Timestamp)
		}()
	}

	response.MessageID = resp.MessageID
	response.Status = fmt.Sprintf("Poll sent to %s", request.BaseRequest.Phone)
	return response, nil
}

func (service serviceSend) SendPresence(ctx context.Context, request domainSend.PresenceRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendPresence(ctx, request)
	if err != nil {
		return response, err
	}

	_, err = service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return response, err
	}

	// In legacy, we just called SendPresence on client, which sets global presence
	// The adapter SendPresence expects chatID (for composing) or empty?
	// Actually, the legacy code called `cli.SendPresence(ctx, types.Presence(request.Type))` which is global presence (Available/Unavailable).
	// But our ChannelAdapter interface `SendPresence` signature is `SendPresence(ctx context.Context, chatID string, typing bool) error`.
	// This seems to mismatch. The legacy `SendPresence` endpoint was likely for "Available/Unavailable" status.
	// We might need to extend adapter or reuse `SendPresence` if it supports empty chatID for global status?
	// Looking at WhatsAppAdapter implementation: it calls `SendChatPresence`. So it's for typing indicators.
	// It seems we confuse "Global Presence (Online/Offline)" with "Chat Presence (Typing/Paused)".
	// Legacy `SendPresence` used `types.Presence` (Available/Unavailable).
	// Legacy `SendChatPresence` used `types.ChatPresence` (Composing/Paused).
	// CHECK: ChannelAdapter interface only has `SendPresence(ctx, chatID, typing)`.
	// We might need an `SetStatus` or `SetOnline` method?
	// For now, let's look at `SendChatPresence` refactor first, which maps to `adapter.SendPresence`.

	// Since we don't have a direct mapping for "Set Online/Offline" in adapter yet (unless I missed it),
	// I will comment out the implementation or use a placeholder if appropriate, OR better:
	// If `request.Type` maps to available/unavailable, we probably need a new method `SetDetailedStatus`?
	// But wait, `SendPresence` in adapter takes `typing bool`.
	// Let's assume for this refactor we only support Chat Presence (Typing) fully via `SendChatPresence`.
	// The `SendPresence` (online/offline) might be deprecated or needs a new adapter method.
	// Let's check `SendChatPresence` below.

	// ... temporary skip or use what we have ...
	// Actually, let's leave SendPresence as is but unimplemented or error until we add `SetOnline` to adapter?
	// OR, if `SendPresence` was for typing, we use it. But the request has `Type` string.

	// Let's focus on `SendChatPresence` which is clearly "Composing/Paused".
	return response, fmt.Errorf("SendPresence (Online/Offline) not supported in adapter yet")
}

func (service serviceSend) SendChatPresence(ctx context.Context, request domainSend.ChatPresenceRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendChatPresence(ctx, request)
	if err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.BaseRequest.Token)
	if err != nil {
		return response, err
	}

	recipient := request.Phone
	if !strings.Contains(recipient, "@") {
		recipient = recipient + "@s.whatsapp.net"
	}

	var typing bool
	var messageID string
	var statusMessage string

	switch request.Action {
	case "start":
		typing = true
		messageID = "chat-presence-start"
		statusMessage = fmt.Sprintf("Send chat presence start typing success %s", request.Phone)
	case "stop":
		typing = false
		messageID = "chat-presence-stop"
		statusMessage = fmt.Sprintf("Send chat presence stop typing success %s", request.Phone)
	default:
		return response, fmt.Errorf("invalid action: %s. Must be 'start' or 'stop'", request.Action)
	}

	err = adapter.SendPresence(ctx, recipient, typing, false)
	if err != nil {
		return response, err
	}

	response.MessageID = messageID
	response.Status = statusMessage
	return response, nil
}

func (service serviceSend) SendSticker(ctx context.Context, request domainSend.StickerRequest) (response domainSend.GenericResponse, err error) {
	// Validate request
	err = validations.ValidateSendSticker(ctx, request)
	if err != nil {
		return response, err
	}

	adapter, err := service.getAdapterForToken(ctx, request.BaseRequest.Token)
	if err != nil {
		return response, err
	}

	recipient := request.Phone
	if !strings.Contains(recipient, "@") {
		recipient = recipient + "@s.whatsapp.net"
	}

	var (
		stickerPath  string
		deletedItems []string
		stickerBytes []byte
	)

	// Resolve absolute base directory for send items
	absBaseDir, err := filepath.Abs(globalConfig.PathSendItems)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to resolve base directory: %v", err))
	}

	defer func() {
		// Delete temporary files
		for _, path := range deletedItems {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				logrus.Warnf("Failed to cleanup temporary file %s: %v", path, err)
			}
		}
	}()

	// Handle sticker from URL or file
	if request.StickerURL != nil && *request.StickerURL != "" {
		// Download sticker from URL
		imageData, _, err := pkgUtils.DownloadImageFromURL(*request.StickerURL)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to download sticker from URL: %v", err))
		}

		// Create safe temporary file within base dir
		f, err := os.CreateTemp(absBaseDir, "sticker_*")
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to create temp file: %v", err))
		}
		stickerPath = f.Name()
		if _, err := f.Write(imageData); err != nil {
			f.Close()
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to write sticker: %v", err))
		}
		_ = f.Close()
		deletedItems = append(deletedItems, stickerPath)
	} else if request.Sticker != nil {
		// Create safe temporary file within base dir
		f, err := os.CreateTemp(absBaseDir, "sticker_*")
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to create temp file: %v", err))
		}
		stickerPath = f.Name()
		_ = f.Close()

		// Save uploaded file to safe path
		err = fasthttp.SaveMultipartFile(request.Sticker, stickerPath)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to save sticker: %v", err))
		}
		deletedItems = append(deletedItems, stickerPath)
	}

	// Convert image to WebP format for sticker (512x512 max size)
	srcImage, err := imaging.Open(stickerPath)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to open image for sticker conversion: %v", err))
	}

	// Rescue existing resizing and conversion logic
	bounds := srcImage.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width > 512 || height > 512 {
		if width > height {
			srcImage = imaging.Resize(srcImage, 512, 0, imaging.Lanczos)
		} else {
			srcImage = imaging.Resize(srcImage, 0, 512, imaging.Lanczos)
		}
	}

	// Convert to WebP using external command (ffmpeg or cwebp)
	webpPath := filepath.Join(absBaseDir, fmt.Sprintf("sticker_%s.webp", fiberUtils.UUIDv4()))
	deletedItems = append(deletedItems, webpPath)

	// First save as PNG temporarily
	pngPath := filepath.Join(absBaseDir, fmt.Sprintf("temp_%s.png", fiberUtils.UUIDv4()))
	deletedItems = append(deletedItems, pngPath)

	err = imaging.Save(srcImage, pngPath)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to save temporary PNG: %v", err))
	}

	// Try to use ffmpeg first (most common), then cwebp
	var convertCmd *exec.Cmd

	// Add execution timeout for conversion
	convCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	// Check if ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		// Use ffmpeg to convert to WebP with transparency support, overwrite if exists
		convertCmd = exec.CommandContext(convCtx, "ffmpeg", "-y", "-i", pngPath, "-vcodec", "libwebp", "-lossless", "0", "-compression_level", "6", "-q:v", "60", "-preset", "default", "-loop", "0", "-an", "-vsync", "0", webpPath)
	} else if _, err := exec.LookPath("cwebp"); err == nil {
		// Use cwebp as fallback
		convertCmd = exec.CommandContext(convCtx, "cwebp", "-q", "60", "-o", webpPath, pngPath)
	} else {
		// If neither tool is available, return error
		return response, pkgError.InternalServerError("neither ffmpeg nor cwebp is installed for WebP conversion")
	}

	var stderr bytes.Buffer
	convertCmd.Stderr = &stderr

	if err := convertCmd.Run(); err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to convert sticker to WebP: %v, stderr: %s", err, stderr.String()))
	}

	// Read the WebP file
	stickerBytes, err = os.ReadFile(webpPath)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to read WebP sticker: %v", err))
	}

	mediaUpload := wsDomainCommon.MediaUpload{
		Data:     stickerBytes,
		FileName: "sticker.webp",
		MimeType: "image/webp",
		Type:     wsDomainCommon.MediaTypeSticker,
		Caption:  "",
	}

	resp, err := adapter.SendMedia(ctx, recipient, mediaUpload, "")
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to send sticker: %v", err))
	}

	// Store sent message
	repo, err := service.getChatStorageForToken(ctx, request.BaseRequest.Token)
	if err == nil {
		go func() {
			content := "💟 Sticker"
			_ = repo.StoreSentMessageWithContext(context.Background(), resp.MessageID, adapter.ID(), recipient, content, resp.Timestamp)
		}()
	}

	response.MessageID = resp.MessageID
	response.Status = fmt.Sprintf("Sticker sent to %s", request.Phone)
	return response, nil
}
