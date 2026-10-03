package core

import "testing"

func TestLooksLikeHongguoShare(t *testing.T) {
	samples := []struct {
		text string
		want bool
	}{
		{text: "【红果免费短剧】长按复制此条消息，打开【红果免费短剧】即可观看《谁说没灵根不能修仙》https://novelquickapp.com/s/abc123", want: true},
		{text: "https://hongguoduanju.com/detail?series_id=7512345678901234567", want: true},
		{text: "看这部《随机购物清单》明天记得买菜", want: false},
		{text: "https://example.com/s/not-a-share", want: false},
		{text: "", want: false},
	}
	for _, sample := range samples {
		if got := looksLikeHongguoShare(sample.text); got != sample.want {
			t.Fatalf("looksLikeHongguoShare(%q)=%v want %v", sample.text, got, sample.want)
		}
	}
}

func TestSeriesIDFromHongguoShare(t *testing.T) {
	text := `《测试剧》https://www.hongguoduanju.com/detail?series_id=7512345678901234567。`
	title := extractHongguoShareTitle(text)
	if title != "测试剧" {
		t.Fatalf("title: %q", title)
	}
	urls := extractHongguoShareURLs(text)
	if len(urls) != 1 || urls[0] != "https://www.hongguoduanju.com/detail?series_id=7512345678901234567" {
		t.Fatalf("urls: %v", urls)
	}
	id := seriesIDFromHongguoShareText(text)
	if id != "7512345678901234567" {
		t.Fatalf("id: %q", id)
	}
	if got := seriesIDFromHongguoShareURL("https://fanqienovel.com/page/7512345678901234567"); got != "7512345678901234567" {
		t.Fatalf("path id: %q", got)
	}
	if got := seriesIDFromHongguoShareHTML(`{"video_data":{"series_id_str":"7512345678901234567"}}`); got != "7512345678901234567" {
		t.Fatalf("html id: %q", got)
	}
}

func TestPickHongguoShareDrama(t *testing.T) {
	dramas := []Drama{
		{ID: "hongguo:1", Title: "谁说没灵根不能修仙的？之无灵证道第23季"},
		{ID: "hongguo:2", Title: "谁说没灵根不能修仙的？之无灵证道第22季"},
	}
	picked := pickHongguoShareDrama("谁说没灵根不能修仙的？之无灵证道第23季", dramas)
	if picked.ID != "hongguo:1" {
		t.Fatalf("exact pick: %+v", picked)
	}
	if pickHongguoShareDrama("谁说没灵根不能修仙的之无灵证道", dramas).ID != "" {
		t.Fatal("ambiguous prefix should be rejected")
	}
	single := pickHongguoShareDrama("谁说没灵根不能修仙的之无灵证道第23季", []Drama{dramas[0]})
	if single.ID != "hongguo:1" {
		t.Fatalf("single prefix: %+v", single)
	}
}
