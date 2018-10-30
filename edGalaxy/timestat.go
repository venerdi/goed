package edGalaxy

import (
	"fmt"
	"time"
)

type TimeVisitStat struct {
	Timemark   int64 `json:"time_mark"`
	VisitCount int64 `json:"visit_count"`
}

type TimeVisitStatCollector struct {
	/*
		Maximum history to keep
	*/
	MaxMarks int `json:"max_marks"`
	/*
		Collector timeframe in seconds
	*/
	Timeframe int64            `json:"timeframe"`
	Visits    []*TimeVisitStat `json:"Visits"`
}

func NewTimeVisitStatCollector(maxMarks int, frame int64) *TimeVisitStatCollector {
	return &TimeVisitStatCollector{
		MaxMarks:  maxMarks,
		Timeframe: frame,
		Visits:    make([]*TimeVisitStat, 0)}
}

func (c *TimeVisitStatCollector) NoteVisit(timestamp time.Time) {
	timemark := timestamp.Unix() / c.Timeframe
	c.noteVisitByTimemark(timemark)
}

func dumpVisits(visits []*TimeVisitStat) {
	fmt.Println("----------")
	for i, v := range visits {
		fmt.Printf("%d: (%d, %d)\n", i, v.Timemark, v.VisitCount)
	}
	fmt.Println("=========")
}

func (c *TimeVisitStatCollector) noteVisitByTimemark(timemark int64) {
	for _, v := range c.Visits {
		if v.Timemark == timemark {
			v.VisitCount++
			return
		}
	}
	// we dont have such mark in the history,
	// need a new one if it fits
	for i, v := range c.Visits {
		if timemark > v.Timemark {
			tmp := append([]*TimeVisitStat{&TimeVisitStat{timemark, 1}}, c.Visits[i:]...)
			if i == 0 {
				c.Visits = tmp
			} else {
				c.Visits = append(c.Visits[:i], tmp...)
			}

			if len(c.Visits) > c.MaxMarks {
				c.Visits = c.Visits[:c.MaxMarks]
			}
			return
		}
	}
	if len(c.Visits) < c.MaxMarks {
		// append it to the end
		c.Visits = append(c.Visits, &TimeVisitStat{timemark, 1})
	}
}
