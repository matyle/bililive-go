package weibolive

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/hr3lxphr6j/bililive-go/src/pkg/utils"
	"github.com/hr3lxphr6j/requests"
	"github.com/tidwall/gjson"

	"github.com/hr3lxphr6j/bililive-go/src/live"
	"github.com/hr3lxphr6j/bililive-go/src/live/internal"
)

const (
	domain = "weibo.com"
	cnName = "微博直播"

	liveurl = "https://weibo.com/l/!/2/wblive/room/show_pc_live.json?live_id="
)

func init() {
	live.Register(domain, new(builder))
}

type builder struct{}

func (b *builder) Build(url *url.URL, opt ...live.Option) (live.Live, error) {
	return &Live{
		BaseLive: internal.NewBaseLive(url, opt...),
	}, nil
}

type Live struct {
	internal.BaseLive
	roomID string
}

func (l *Live) getRoomInfo() ([]byte, error) {
	paths := strings.Split(l.Url.Path, "/")
	if len(paths) < 5 {
		return nil, live.ErrRoomUrlIncorrect
	}
	roomid := paths[5]
	l.roomID = roomid

	resp, err := requests.Get(liveurl+roomid,
		live.CommonUserAgent,
		requests.Headers(map[string]interface{}{
			"Referer": l.Url,
		}))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, live.ErrRoomNotExist
	}
	body, err := resp.Bytes()
	if err != nil || gjson.GetBytes(body, "error_code").Int() != 0 {
		return nil, live.ErrRoomNotExist
	}
	return body, nil
}

func (l *Live) GetInfo() (info *live.Info, err error) {
	body, err := l.getRoomInfo()
	if err != nil {
		return nil, live.ErrRoomNotExist
	}
	info = &live.Info{
		Live:         l,
		HostName:     gjson.GetBytes(body, "data.user.screenName").String(),
		RoomName:     gjson.GetBytes(body, "data.title").String(),
		Status:       gjson.GetBytes(body, "data.status").String() == "1",
		CustomLiveId: "weibolive/" + l.roomID,
	}
	return info, nil
}

func (l *Live) GetStreamUrls() (us []*url.URL, err error) {
	body, err := l.getRoomInfo()
	if err != nil {
		return nil, live.ErrRoomNotExist
	}

	streamurl := gjson.GetBytes(body, "data.live_origin_flv_url").String()
	return utils.GenUrls(streamurl)
}

func (l *Live) GetPlatformCNName() string {
	return cnName
}
