package routes

import (
	"EverythingSuckz/fsb/internal/bot"
	"EverythingSuckz/fsb/internal/store"
	"EverythingSuckz/fsb/internal/stream"
	"EverythingSuckz/fsb/internal/types"
	"EverythingSuckz/fsb/internal/utils"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	range_parser "github.com/quantumsheep/range-parser"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
)

var log *zap.Logger

func (e *allRoutes) LoadHome(r *Route) {
	log = e.log.Named("Stream")
	defer log.Info("Loaded stream route")
	r.Engine.GET("/stream/:messageID", getStreamRoute)
	r.Engine.HEAD("/stream/:messageID", getStreamRoute)
}

// resolveFile validates the request (hash, revoked, expired) and loads file metadata.
// On failure it returns an HTTP status and message; status == 0 means success.
func resolveFile(ctx *gin.Context) (file *types.File, rec *store.FileRecord, worker *bot.Worker, messageID int, status int, msg string) {
	messageID, err := strconv.Atoi(ctx.Param("messageID"))
	if err != nil {
		return nil, nil, nil, 0, http.StatusBadRequest, err.Error()
	}
	authHash := ctx.Query("hash")
	if authHash == "" {
		return nil, nil, nil, 0, http.StatusBadRequest, "missing hash param"
	}

	worker = bot.GetNextWorker()
	file, err = utils.TimeFuncWithResult(log, "FileFromMessage", func() (*types.File, error) {
		return utils.FileFromMessage(ctx, worker.Client, messageID)
	})
	if err != nil {
		return nil, nil, nil, messageID, http.StatusBadRequest, err.Error()
	}
	expectedHash := utils.PackFile(file.FileName, file.FileSize, file.MimeType, file.ID)
	if !utils.CheckHash(authHash, expectedHash) {
		return nil, nil, nil, messageID, http.StatusBadRequest, "invalid hash"
	}

	// Links created before the database existed have no record and keep working.
	if r, ok := store.Default.GetFile(messageID); ok {
		if r.Revoked {
			return nil, nil, nil, messageID, http.StatusGone, "this link has been revoked by its owner"
		}
		if r.ExpiresAt != 0 && time.Now().Unix() > r.ExpiresAt {
			return nil, nil, nil, messageID, http.StatusGone, "this link has expired"
		}
		if store.Default.IsBanned(r.OwnerID) {
			return nil, nil, nil, messageID, http.StatusForbidden, "this link is unavailable"
		}
		if st, m := gateLink(ctx, &r); st != 0 {
			return nil, nil, nil, messageID, st, m
		}
		rec = &r
	}
	return file, rec, worker, messageID, 0, ""
}

func contentDisposition(kind, name string) string {
	if v := mime.FormatMediaType(kind, map[string]string{"filename": name}); v != "" {
		return v
	}
	return kind
}

// riskyInline reports mime types that must never be rendered inline from our origin.
func riskyInline(m string) bool {
	m = strings.ToLower(m)
	return strings.Contains(m, "html") || strings.Contains(m, "svg") ||
		strings.Contains(m, "xml") || strings.Contains(m, "javascript")
}

func getStreamRoute(ctx *gin.Context) {
	w := ctx.Writer
	r := ctx.Request

	file, _, worker, messageID, status, msg := resolveFile(ctx)
	if status != 0 {
		http.Error(w, msg, status)
		return
	}
	name := utils.DisplayName(file.FileName, file.MimeType, file.ID)

	disposition := "inline"
	if ctx.Query("d") == "true" || riskyInline(file.MimeType) {
		disposition = "attachment"
	}
	ctx.Header("X-Content-Type-Options", "nosniff")

	// for photo messages
	if file.FileSize == 0 {
		res, err := worker.Client.API().UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: file.Location,
			Offset:   0,
			Limit:    1024 * 1024,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		result, ok := res.(*tg.UploadFile)
		if !ok {
			http.Error(w, "unexpected response", http.StatusInternalServerError)
			return
		}
		fileBytes := result.GetBytes()
		ctx.Header("Content-Disposition", contentDisposition(disposition, name))
		if r.Method != http.MethodHead {
			ctx.Data(http.StatusOK, file.MimeType, fileBytes)
			store.Default.RecordDownload(messageID, int64(len(fileBytes)), true)
		}
		return
	}

	ctx.Header("Accept-Ranges", "bytes")
	var start, end int64
	statusCode := http.StatusOK
	rangeHeader := r.Header.Get("Range")

	if rangeHeader == "" {
		start = 0
		end = file.FileSize - 1
	} else {
		ranges, err := range_parser.Parse(file.FileSize, rangeHeader)
		if err != nil || len(ranges) == 0 {
			ctx.Header("Content-Range", fmt.Sprintf("bytes */%d", file.FileSize))
			http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		start = ranges[0].Start
		end = ranges[0].End
		ctx.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, file.FileSize))
		log.Debug("Content-Range", zap.Int64("start", start), zap.Int64("end", end), zap.Int64("fileSize", file.FileSize))
		statusCode = http.StatusPartialContent
	}

	contentLength := end - start + 1
	mimeType := file.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	ctx.Header("Content-Type", mimeType)
	ctx.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	ctx.Header("Content-Disposition", contentDisposition(disposition, name))
	// Headers must be complete before the status line is written.
	w.WriteHeader(statusCode)

	if r.Method == http.MethodHead {
		return
	}
	pipe, err := stream.NewStreamPipe(ctx, worker.Client, file.Location, start, end, log)
	if err != nil {
		log.Error("Failed to create stream pipe", zap.Error(err))
		return
	}
	defer pipe.Close()
	n, err := io.CopyN(w, pipe, contentLength)
	if err != nil && !utils.IsClientDisconnectError(err) {
		log.Error("Error while copying stream", zap.Error(err))
	}
	// A transfer starting at byte 0 counts as one download; seeks only add bandwidth.
	store.Default.RecordDownload(messageID, n, start == 0)
}
