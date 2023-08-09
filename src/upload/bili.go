package upload

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/imroc/req/v3"
	"github.com/matyle/bililive-go/src/configs"
	"github.com/matyle/bililive-go/src/pkg/zaplogger"
	"github.com/panjf2000/ants/v2"
	"github.com/robfig/cron"
	"github.com/schollz/progressbar/v3"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

type BiliUpload struct {
	cookie string
	csrf   string
	client *req.Client

	threadNum int
	partChan  chan Part
	chunks    int64

	config *configs.BiliupConfig

	log *zap.Logger
}

type BiliUploads struct {
	BiliUploads []*BiliUpload
	CronService *cron.Cron
	Configs     []*configs.BiliupConfig
	log         *zap.Logger
	files       *MediaFiles
}

// 支持上传到多个 bilibili 账号
func NewBiliUPLoads(confs []*configs.BiliupConfig, threadNum int) *BiliUploads {
	logFile := fmt.Sprintf("%s/upload.log", configs.NewConfig().Log.OutPutFolder)
	log := zaplogger.GetFileLogger(logFile).With(zap.String("pkg", "upload")).With(zap.String("users uploads", "all"))
	if len(confs) == 0 {
		panic("cookie文件不存在,请先登录")
	}
	var biliUploads []*BiliUpload
	for _, v := range confs {
		biliUploads = append(biliUploads, newBiliUPLoad(v, threadNum))
	}

	// cronService := cron.New(cron.(), cron.WithLocation(time.UTC), cron.WithParser(
	// 	cron.NewParser(cron.Second|cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.DowOptional|cron.Descriptor)))
	cronService := cron.NewWithLocation(time.UTC)
	return &BiliUploads{
		BiliUploads: biliUploads,
		CronService: cronService,
		Configs:     confs,
		files:       NewMediaFiles(configs.NewConfig().VideosPath),
		log:         log,
	}
}

// 启动立即执行一次定时执行，每天凌晨 1 点执行
func (u *BiliUploads) Server(postUploadHandler func(*BiliUploads)) {
	u.Upload(postUploadHandler)
	// 定时执行
	cronSpec := "20 00 17 * * *" // 每天UTC 时间3点00分20秒 北京时间11点00分20执行
	u.CronService.AddFunc(cronSpec, func() {
		u.Upload(postUploadHandler)
	})
	u.CronService.Start()

}

// 上传视频成功之后，可以删除本地视频
func (u *BiliUploads) Upload(postUploadHandler func(*BiliUploads)) error {
	//扫描文件
	u.files.ScanBiludVideo()
	defer u.files.Clear()

	wg := &sync.WaitGroup{}
	for i, v := range u.BiliUploads {
		wg.Add(1)
		go func(i int, v *BiliUpload) {
			defer wg.Done()
			v.log.Info("开始上传",
				zap.Int("第一个用户", i),
				zap.String("用户名", u.Configs[i].UserName))
			v.uploadFiles(u.files)
		}(i, v)
	}
	wg.Wait()
	u.log.Info("全部上传完成，开始执行后续操作")
	if postUploadHandler != nil {
		postUploadHandler(u)
	}
	return nil
}

func newBiliUPLoad(config *configs.BiliupConfig, threadNum int) *BiliUpload {
	if config.CookiePath == "" {
		panic("cookie文件不存在,请先登录")
	}
	var cookieinfo BiliCookie
	loginInfo, err := os.ReadFile(config.CookiePath)
	if err != nil || len(loginInfo) == 0 {
		panic("cookie文件不存在,请先登录")
	}

	logFile := fmt.Sprintf("%s/%s.log", configs.NewConfig().Log.OutPutFolder, config.UserName)
	logger := zaplogger.GetFileLogger(logFile).
		With(zap.String("pkg", "upload")).
		With(zap.String("username", config.UserName))
	_ = json.Unmarshal(loginInfo, &cookieinfo)
	var cookie string
	var csrf string
	for _, v := range cookieinfo.Data.CookieInfo.Cookies {
		cookie += v.Name + "=" + v.Value + ";"
		if v.Name == "bili_jct" {
			csrf = v.Value
		}
	}
	var client = req.C().SetCommonHeaders(map[string]string{
		"user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36 Edg/105.0.1343.53",
		"cookie":     cookie,
		"Connection": "keep-alive",
	})
	resp, _ := client.R().Get("https://api.bilibili.com/x/web-interface/nav")
	uname := gjson.ParseBytes(resp.Bytes()).Get("data.uname").String()
	if uname == "" {
		panic("cookie失效,请重新登录")
	}
	// log.Printf("%s 登录成功", uname)
	logger.Info("登录成功", zap.String("uname", uname))
	return &BiliUpload{
		cookie:    cookie,
		csrf:      csrf,
		client:    client,
		threadNum: threadNum,
		config:    config,
		log:       logger,
	}
}

func (u *BiliUpload) uploadCover(path string) string {
	if path == "" {
		return ""
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		u.log.Fatal("读取封面失败", zap.Error(err))
	}
	var base64Encoding string
	mimeType := http.DetectContentType(bytes)
	switch mimeType {
	case "image/jpeg", "image/jpg":
		base64Encoding = "data:image/jpeg;base64,"
	case "image/png":
		base64Encoding = "data:image/png;base64,"
	case "image/gif":
		base64Encoding = "data:image/gif;base64,"
	default:
		u.log.Fatal("不支持的图片格式")
	}
	base64Encoding += base64.StdEncoding.EncodeToString(bytes)
	var coverinfo CoverInfo
	u.client.R().SetFormDataFromValues(url.Values{
		"cover": {base64Encoding},
		"csrf":  {u.csrf},
	}).SetResult(&coverinfo).Post("https://member.bilibili.com/x/vu/web/cover/up")
	return coverinfo.Data.Url
}

func (u *BiliUpload) uploadFiles(files *MediaFiles) {
	for filePath, video := range files.uploadingVideo {
		u.log.Info("开始上传视频", zap.String("视频名称", video.fileName))
		err := u.uploadReleseFile(filePath, video)
		if err != nil {
			u.log.Error("上传或者发布失败", zap.Error(err), zap.String("视频名称", video.fileName))
			continue
		}
		u.log.Info("上传成功", zap.String("视频名称", video.fileName))
		files.successVideo[filePath] = struct{}{}
	}
}

func (u *BiliUpload) uploadReleseFile(filePath string, upVideo *localVideo) error {
	var preupinfo PreUpInfo
	u.client.R().SetQueryParams(map[string]string{
		"probe_version": "20211012",
		"upcdn":         "bda2",
		"zone":          "cs",
		"name":          upVideo.fileName,
		"r":             "upos",
		"profile":       "ugcfx/bup",
		"ssl":           "0",
		"version":       "2.10.4.0",
		"build":         "2100400",
		"size":          strconv.FormatInt(upVideo.videoSize, 10),
		"webVersion":    "2.0.0",
	}).SetResult(&preupinfo).Get("https://member.bilibili.com/preupload")

	file, err := os.Open(filePath)
	if err != nil {
		u.log.Error("打开文件失败", zap.Error(err))
		return err
	}
	defer file.Close()
	upVideo.uploadBaseUrl = fmt.Sprintf("https:%s/%s", preupinfo.Endpoint, strings.Split(preupinfo.UposUri, "//")[1])
	upVideo.biliFileName = strings.Split(strings.Split(strings.Split(preupinfo.UposUri, "//")[1], "/")[1], ".")[0]
	upVideo.chunkSize = preupinfo.ChunkSize
	upVideo.auth = preupinfo.Auth
	upVideo.bizId = preupinfo.BizId
	err = u.upload(upVideo, file)
	if err != nil {
		u.log.Error("上传失败", zap.Error(err))
		return err
	}

	title := time.Now().Format("2006-01-02") + "_" + u.config.VideoTitlePrefix + "_" + upVideo.fileName
	var addreq = BiliReq{
		Copyright:    u.config.UpType,
		Cover:        upVideo.coverUrl,
		Title:        title,
		Tid:          u.config.Tid,
		Tag:          u.config.Tag,
		DescFormatId: 16,
		Desc:         u.config.VideoDesc,
		Source:       u.config.Source,
		Dynamic:      "",
		Interactive:  0,
		Videos: []Video{
			{
				Filename: upVideo.biliFileName,
				Title:    upVideo.fileName,
				Desc:     "",
				Cid:      preupinfo.BizId,
			},
		},
		ActReserveCreate: 0,
		NoDisturbance:    0,
		NoReprint:        1,
		Subtitle: Subtitle{
			Open: 0,
			Lan:  "",
		},
		Dolby:         0,
		LosslessMusic: 0,
		Csrf:          u.csrf,
	}
	_ = addreq
	//TODO add retry logic
	resp, err := u.client.R().SetQueryParams(map[string]string{
		"csrf": u.csrf,
	}).SetBodyJsonMarshal(addreq).Post("https://member.bilibili.com/x/vu/web/add/v3")
	if err != nil || resp.StatusCode != 200 {
		u.log.Error("发布失败", zap.Error(err), zap.String("resp", resp.String()))
		return err
	}
	var res UploadResp
	if err := json.Unmarshal([]byte(resp.String()), &res); err != nil {
		u.log.Error("Unmarshal result error", zap.Error(err))
	}

	if res.Code != 0 {
		u.log.Error("resp", zap.String("resp", resp.String()))
	}
	return nil
}

func (u *BiliUpload) upload(upVideo *localVideo, file *os.File) error {
	defer ants.Release()
	var upinfo UpInfo
	u.client.SetCommonHeader(
		"X-Upos-Auth", upVideo.auth).R().
		SetQueryParams(map[string]string{
			"uploads":       "",
			"output":        "json",
			"profile":       "ugcfx/bup",
			"filesize":      strconv.FormatInt(upVideo.videoSize, 10),
			"partsize":      strconv.FormatInt(upVideo.chunkSize, 10),
			"biz_id":        strconv.FormatInt(upVideo.bizId, 10),
			"meta_upos_uri": u.getMetaUposUri(),
		}).SetResult(&upinfo).Post(upVideo.uploadBaseUrl)
	upVideo.uploadId = upinfo.UploadId
	u.chunks = int64(math.Ceil(float64(upVideo.videoSize) / float64(upVideo.chunkSize)))
	var reqjson = new(ReqJson)
	chunk := 0
	start := 0
	end := 0
	bar := progressbar.NewOptions(int(upVideo.videoSize/1024/1024),
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionSetItsString("MB"),
		progressbar.OptionSetDescription("视频上传中..."),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowIts(),
	)
	wg := &sync.WaitGroup{}
	u.partChan = make(chan Part, u.chunks)
	go func() {
		for p := range u.partChan {
			reqjson.Parts = append(reqjson.Parts, p)
		}
	}()
	p, _ := ants.NewPool(u.threadNum)
	defer p.Release()
	for {
		buf := make([]byte, upVideo.chunkSize)
		size, err := file.Read(buf)
		if err != nil && err != io.EOF {
			break
		}
		buf = buf[:size]
		if size > 0 {
			wg.Add(1)
			end += size
			_ = p.Submit(u.uploadPartWrapper(wg, chunk, start, end, size, buf, upVideo, bar))
			buf = nil
			start += size
			chunk++
		}
		if err == io.EOF {
			break
		}
	}
	wg.Wait()
	close(u.partChan)
	u.log.Debug("success part chan")
	jsonString, _ := json.Marshal(&reqjson)
	u.client.R().SetHeaders(map[string]string{
		"Content-Type": "application/json",
		"Origin":       "https://member.bilibili.com",
		"Referer":      "https://member.bilibili.com/",
	}).SetQueryParams(map[string]string{
		"output":   "json",
		"profile":  "ugcfx/bup",
		"name":     upVideo.fileName,
		"uploadId": upVideo.uploadId,
		"biz_id":   strconv.FormatInt(upVideo.bizId, 10),
	}).SetBodyString(string(jsonString)).SetResult(&upinfo).SetRetryCount(5).AddRetryHook(func(resp *req.Response, err error) {
		u.log.Debug("重试发送分片确认请求")
		return
	}).
		AddRetryCondition(func(resp *req.Response, err error) bool {
			return err != nil || resp.StatusCode != 200
		}).Post(upVideo.uploadBaseUrl)
	return nil
}

type taskFunc func()

func (u *BiliUpload) uploadPartWrapper(wg *sync.WaitGroup, chunk int, start, end, size int, buf []byte, upVideo *localVideo, bar *progressbar.ProgressBar) taskFunc {
	return func() {
		defer wg.Done()
		resp, _ := u.client.R().SetHeaders(map[string]string{
			"Content-Type":   "application/octet-stream",
			"Content-Length": strconv.Itoa(size),
		}).SetQueryParams(map[string]string{
			"partNumber": strconv.Itoa(chunk + 1),
			"uploadId":   upVideo.uploadId,
			"chunk":      strconv.Itoa(chunk),
			"chunks":     strconv.Itoa(int(u.chunks)),
			"size":       strconv.Itoa(size),
			"start":      strconv.Itoa(start),
			"end":        strconv.Itoa(end),
			"total":      strconv.FormatInt(upVideo.videoSize, 10),
		}).SetBodyBytes(buf).SetRetryCount(5).AddRetryHook(func(resp *req.Response, err error) {
			// log.Println("重试发送分片", chunk)
			u.log.Debug("uploadPartWrapper",
				zap.Int("重试发送分片", chunk))
			return
		}).
			AddRetryCondition(func(resp *req.Response, err error) bool {
				return err != nil || resp.StatusCode != 200
			}).Put(upVideo.uploadBaseUrl)
		bar.Add(len(buf) / 1024 / 1024)
		if resp.StatusCode != 200 {
			// log.Println("分片", chunk, "上传失败", resp.StatusCode, "size", size)
			u.log.Error("uploadPartWrapper",
				zap.Int("分片", chunk),
				zap.Int("StatusCode", resp.StatusCode),
				zap.Int("size", size),
				zap.Int("start", start),
				zap.Int("end", end))
			return
		}
		u.partChan <- Part{
			PartNumber: int64(chunk + 1),
			ETag:       "etag",
		}
	}
}

func (u *BiliUpload) getMetaUposUri() string {
	var metaUposUri PreUpInfo
	u.client.R().SetQueryParams(map[string]string{
		"name":       "file_meta.txt",
		"size":       "2000",
		"r":          "upos",
		"profile":    "fxmeta/bup",
		"ssl":        "0",
		"version":    "2.10.4",
		"build":      "2100400",
		"webVersion": "2.0.0",
	}).SetResult(&metaUposUri).Get("https://member.bilibili.com/preupload")
	return metaUposUri.UposUri
}
