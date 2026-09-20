package meta

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/abema/go-mp4"
)

const qtEpochOffset = 2082844800

var copyrightDay = mp4.BoxType{0xa9, 'd', 'a', 'y'}

type VideoProvider struct{ MVHDLocation *time.Location }

func (VideoProvider) Supports(f Format) bool { return f == FormatMOV || f == FormatMP4 }

func (VideoProvider) Provides() []string { return videoFields }

func (p VideoProvider) Extract(ctx context.Context, base *Record, reader io.ReadSeeker) (*Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r := base.clone()
	var keys []string
	var keyed, day, xml string
	var mvhd *mp4.Mvhd
	_, err := mp4.ReadBoxStructure(contextReader{ctx, reader}, func(h *mp4.ReadHandle) (interface{}, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(h.Path) > 32 {
			return nil, fmt.Errorf("ISO-BMFF nesting exceeds 32")
		}
		if h.BoxInfo.Size > 1<<30 && h.BoxInfo.Type != mp4.BoxTypeMdat() {
			return nil, fmt.Errorf("box %s too large", h.BoxInfo.Type)
		}
		if !base.fields.captured() && h.BoxInfo.Type != mp4.BoxTypeMoov() && h.BoxInfo.Type != mp4.BoxTypeMvhd() {
			return nil, nil
		}
		switch h.BoxInfo.Type {
		case mp4.BoxTypeMoov(), mp4.BoxTypeUdta(), mp4.BoxTypeMeta(), mp4.BoxTypeIlst():
			_, expandErr := h.Expand()
			return nil, expandErr
		case mp4.BoxTypeKeys():
			box, _, readErr := h.ReadPayload()
			if readErr != nil {
				return nil, readErr
			}
			for _, key := range box.(*mp4.Keys).Entries {
				keys = append(keys, string(key.KeyValue))
			}
		case mp4.BoxTypeMvhd():
			box, _, readErr := h.ReadPayload()
			if readErr != nil {
				return nil, readErr
			}
			mvhd = box.(*mp4.Mvhd)
		case mp4.BoxTypeData():
			box, _, readErr := h.ReadPayload()
			if readErr != nil {
				return nil, readErr
			}
			value := strings.TrimSpace(strings.TrimRight(string(box.(*mp4.Data).Data), "\x00"))
			if len(h.Path) >= 2 {
				parent := h.Path[len(h.Path)-2]
				if parent == copyrightDay {
					day = value
				} else {
					index := int(binary.BigEndian.Uint32(parent[:]))
					if index > 0 && index <= len(keys) && keys[index-1] == "com.apple.quicktime.creationdate" {
						keyed = value
					}
				}
			}
		case copyrightDay:
			_, expandErr := h.Expand()
			return nil, expandErr
		case mp4.StrToBoxType("uuid"):
			if h.BoxInfo.Size <= 1<<20 {
				var data bytes.Buffer
				if _, readErr := h.ReadData(&data); readErr == nil {
					xml = findXMLDate(data.String())
				}
			}
		default:
			if len(h.Path) >= 2 && h.Path[len(h.Path)-2] == mp4.BoxTypeIlst() {
				index := int(binary.BigEndian.Uint32(h.BoxInfo.Type[:]))
				if index <= 0 || index > len(keys) || keys[index-1] != "com.apple.quicktime.creationdate" {
					return nil, nil
				}
				box, _, readErr := h.ReadPayload()
				if readErr != nil {
					return nil, readErr
				}
				keyed = strings.TrimSpace(strings.TrimRight(string(box.(*mp4.Item).Data.Data), "\x00"))
			}
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	if mvhd != nil && mvhd.Timescale > 0 {
		duration := time.Duration(float64(mvhd.GetDuration()) / float64(mvhd.Timescale) * float64(time.Second))
		r.Duration = &duration
	}
	if !base.fields.captured() {
		return r, nil
	}
	for _, candidate := range []struct{ value, source string }{{keyed, "qt:com.apple.quicktime.creationdate"}, {day, "qt:©day"}, {xml, "qt:xml.creationdatevalue"}} {
		if tm, hasOffset, ok := parseVideoTime(candidate.value, p.MVHDLocation); ok {
			r.Captured = &Timestamp{Time: tm, HasOffset: hasOffset, Source: candidate.source}
			if base.fields.Has(FieldRaw) {
				r.Raw[candidate.source] = candidate.value
			}
			return r, nil
		}
	}
	if mvhd == nil {
		return r, nil
	}
	raw := mvhd.GetCreationTime()
	if raw == 0 || raw < qtEpochOffset {
		return r, nil
	}
	location := p.MVHDLocation
	if location == nil {
		location = time.Local
	}
	tm := time.Unix(int64(raw)-qtEpochOffset, 0)
	if location != time.UTC {
		year, month, day := tm.UTC().Date()
		hour, minute, second := tm.UTC().Clock()
		tm = time.Date(year, month, day, hour, minute, second, 0, location)
	} else {
		tm = tm.UTC()
	}
	if tm.Before(time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)) || tm.After(time.Now().Add(24*time.Hour)) {
		return r, nil
	}
	r.Captured = &Timestamp{Time: tm, HasOffset: false, Source: "qt:mvhd.creation_time"}
	return r, nil
}

var (
	videoOffsetLayouts = []string{time.RFC3339Nano, "2006-01-02T15:04:05-0700", "2006:01:02 15:04:05Z07:00"}
	videoLocalLayouts  = []string{"2006-01-02T15:04:05", "2006:01:02 15:04:05"}
)

func parseVideoTime(value string, loc *time.Location) (time.Time, bool, bool) {
	return parseTimestamp(value, "", loc, videoOffsetLayouts, videoLocalLayouts)
}

var xmlDatePattern = regexp.MustCompile(`(?i)<(?:CreationDateValue|CreateDate)>([^<]+)</`)

func findXMLDate(value string) string {
	match := xmlDatePattern.FindStringSubmatch(value)
	if len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}
