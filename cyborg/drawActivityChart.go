package cyborg

import (
	"fmt"
	"github.com/wcharczuk/go-chart"
	"github.com/wcharczuk/go-chart/drawing"
	//	util "github.com/wcharczuk/go-chart/util"
	"errors"
	"github.com/dustin/go-humanize"
	"goed/edGalaxy"
	"io"
	"math"
	"time"
)

var (
	discordBlue     = drawing.Color{R: 114, G: 137, B: 218, A: 255}
	discordWhite    = drawing.Color{R: 255, G: 255, B: 255, A: 255}
	discordGrey     = drawing.Color{R: 153, G: 170, B: 181, A: 255}
	discordDarkGrey = drawing.Color{R: 44, G: 47, B: 51, A: 255}
	discordBlack    = drawing.Color{R: 35, G: 39, B: 42, A: 255}
)

func makeYTicks(values []float64) chart.Ticks {
	if len(values) < 1 {
		return nil
	}

	min := values[0]
	max := values[0]

	for _, v := range values {
		min = math.Min(min, v)
		max = math.Max(max, v)
	}
	nticks := 8

	r := max - min
	tickSize := r / float64(nticks-1)
	l := math.Log10(tickSize)
	tSize := math.Pow(10, math.Round(l))

	ft := tSize * math.Floor(min/tSize)
	ticks := make([]chart.Tick, 0)
	ticks = append(ticks, chart.Tick{Value: ft, Label: humanize.Comma(int64(ft))})
	ft += tSize
	for ; ft < max; ft += tSize {
		ticks = append(ticks, chart.Tick{Value: ft, Label: humanize.Comma(int64(ft))})
	}
	ticks = append(ticks, chart.Tick{Value: ft, Label: humanize.Comma(int64(ft))})
	return ticks

}

func makeDayTicks(stat []*edGalaxy.ActivityStatItem) chart.Ticks {
	l := len(stat)
	if l < 1 {
		return nil
	}

	minTimestamp := stat[0].Timestamp
	maxTimestamp := minTimestamp

	for _, s := range stat {
		minTimestamp = edGalaxy.Min(minTimestamp, s.Timestamp)
		maxTimestamp = edGalaxy.Max(maxTimestamp, s.Timestamp)
	}

	var secondsPerDay int64 = 24 * 60 * 60

	firstDay := (minTimestamp + secondsPerDay - 1) / secondsPerDay
	ticks := make([]chart.Tick, 0)

	dayTick := firstDay * secondsPerDay

	if minTimestamp < dayTick {
		ticks = append(ticks,
			chart.Tick{Value: float64(minTimestamp), Label: ""})
	}
	for ; dayTick < maxTimestamp; dayTick += secondsPerDay {
		ticks = append(ticks,
			chart.Tick{
				Value: float64(dayTick),
				Label: time.Unix(dayTick-1, 0).UTC().Format("2006-01-02")})
	}
	if dayTick-secondsPerDay < maxTimestamp {
		ticks = append(ticks,
			chart.Tick{
				Value: float64(maxTimestamp + 3600),
				Label: "",
			})
	}
	return ticks
}

func makeGridLines(dayTicks chart.Ticks) []chart.GridLine {
	l := len(dayTicks)
	if l < 3 {
		return nil
	}
	gridLines := make([]chart.GridLine, l-2)
	for i, t := range dayTicks[1 : l-1] {
		gridLines[i] = chart.GridLine{Value: t.Value}
	}
	return gridLines
}

func makeJumpsDocksSeries(stat []*edGalaxy.ActivityStatItem) (jumps, docks, times []float64) {
	l := len(stat)
	if l < 1 {
		return
	}
	times = make([]float64, l)
	jumps = make([]float64, l)
	docks = make([]float64, l)

	for i, s := range stat {
		times[i] = float64(s.Timestamp)
		jumps[i] = float64(s.NumJumps)
		docks[i] = float64(s.NumDocks)
	}

	return
}

type activityColorPalette struct{}

func (dp activityColorPalette) BackgroundColor() drawing.Color {
	return discordDarkGrey
}

func (dp activityColorPalette) BackgroundStrokeColor() drawing.Color {
	return discordWhite
}

func (dp activityColorPalette) CanvasColor() drawing.Color {
	return discordGrey.WithAlpha(100)
}

func (dp activityColorPalette) CanvasStrokeColor() drawing.Color {
	return discordWhite
}

func (dp activityColorPalette) AxisStrokeColor() drawing.Color {
	return discordWhite
}

func (dp activityColorPalette) TextColor() drawing.Color {
	return discordWhite
}

func (dp activityColorPalette) GetSeriesColor(index int) drawing.Color {
	return chart.GetDefaultColor(index)
}

func HumanIntValueFormatter(v interface{}) string {
	switch v.(type) {
	case int:
		return humanize.Comma(int64(v.(int)))
	case int64:
		return humanize.Comma(v.(int64))
	case float32:
		return humanize.Comma(int64(v.(float32)))
	case float64:
		return humanize.Comma(int64(v.(float64)))
	default:
		return ""
	}
}

func UTCUnixTimeFormatter(v interface{}) string {
	var tm time.Time

	switch v.(type) {
	case float64:
		tm = time.Unix(int64(v.(float64)), 0)
	default:
		fmt.Printf("fmt unknown\n")
		return ""
	}
	return tm.UTC().Format(time.RFC3339)
}

func DrawChart(stat []*edGalaxy.ActivityStatItem, out io.Writer) error {

	if stat == nil || len(stat) < 2 {
		return errors.New("Insufficient data points")
	}

	stat = stat[1:] // strip last hour - could be 0 at the beginning
	dayTicks := makeDayTicks(stat[1:])

	js, ds, times := makeJumpsDocksSeries(stat)

	graph := chart.Chart{
		Width:  900,
		Height: 450,
		DPI:    72,
		Background: chart.Style{
			Padding: chart.Box{
				Top:    10,
				Left:   5,
				Right:  5,
				Bottom: 5,
			},
		},

		ColorPalette: activityColorPalette{},
		XAxis: chart.XAxis{
			Style:          chart.StyleShow(),
			TickPosition:   chart.TickPositionBetweenTicks,
			ValueFormatter: UTCUnixTimeFormatter,
			Ticks:          dayTicks,
			GridMajorStyle: chart.Style{
				Show:            true,
				StrokeColor:     discordWhite,
				StrokeDashArray: []float64{5.0, 5.0},
				StrokeWidth:     0.5,
			},
			GridLines: makeGridLines(dayTicks),
		},
		YAxis: chart.YAxis{
			Style: chart.StyleShow(),
			//			Ticks:          makeYTicks(js),
			ValueFormatter: HumanIntValueFormatter,
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Style: chart.Style{
					Show:        true,
					StrokeColor: discordBlue,                // chart.ColorBlue,
					FillColor:   discordBlue.WithAlpha(150), // chart.ColorBlue.WithAlpha(100),
				},
				Name:    "Jumps/h",
				XValues: times,
				YValues: js},
			chart.ContinuousSeries{
				Style: chart.Style{
					Show:        true,
					StrokeColor: chart.ColorAlternateGreen,
					FillColor:   chart.ColorAlternateGreen.WithAlpha(100),
				},
				Name:    "Docks/h",
				XValues: times,
				YValues: ds},
		},
	}

	graph.Elements = []chart.Renderable{
		chart.LegendThin(&graph,
			chart.Style{
				FillColor:   discordDarkGrey,
				FontColor:   discordWhite,
				FontSize:    9.0,
				StrokeWidth: chart.DefaultAxisLineWidth})}
	graph.Render(chart.PNG, out)
	return nil
}
