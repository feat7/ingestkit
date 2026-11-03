package benchmark

import (
	"context"
	"errors"
	"time"

	"github.com/feat7/ingestkit/internal/runenv"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func offsetsCaughtUp(ends, committed map[int32]int64) bool {
	if len(ends) == 0 {
		return false
	}
	for partition, end := range ends {
		if end < 0 {
			return false
		}
		offset, ok := committed[partition]
		if end == 0 && (!ok || offset == -1) {
			continue
		}
		if !ok || offset != end {
			return false
		}
	}
	return true
}

// Wait for commit-before-cleanup, not merely row visibility. Otherwise a later
// deliberate broker fault could replay an experiment row that cleanup deleted.
func waitConsumerCaughtUp(ctx context.Context, s *runenv.Stack) error {
	if s.Brokers == "" {
		return errors.New("owned broker probe is required for progress verification")
	}
	probe, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client, err := kgo.NewClient(kgo.SeedBrokers(s.Brokers))
	if err != nil {
		return err
	}
	defer client.Close()
	partitions := []int32{0, 1, 2}
	for {
		list := kmsg.NewPtrListOffsetsRequest()
		topic := kmsg.ListOffsetsRequestTopic{Topic: "ingestkit.events"}
		for _, id := range partitions {
			p := kmsg.NewListOffsetsRequestTopicPartition()
			p.Partition = id
			p.Timestamp = -1
			topic.Partitions = append(topic.Partitions, p)
		}
		list.Topics = []kmsg.ListOffsetsRequestTopic{topic}
		ends := map[int32]int64{}
		committed := map[int32]int64{}
		response, e := client.Request(probe, list)
		if e == nil {
			for _, t := range response.(*kmsg.ListOffsetsResponse).Topics {
				for _, p := range t.Partitions {
					if p.ErrorCode == 0 {
						ends[p.Partition] = p.Offset
					}
				}
			}
		}
		fetch := kmsg.NewPtrOffsetFetchRequest()
		fetch.Group = "ingestkit-consumer"
		fetch.Topics = []kmsg.OffsetFetchRequestTopic{{Topic: "ingestkit.events", Partitions: partitions}}
		group := kmsg.NewOffsetFetchRequestGroup()
		group.Group = "ingestkit-consumer"
		group.Topics = []kmsg.OffsetFetchRequestGroupTopic{{Topic: "ingestkit.events", Partitions: partitions}}
		fetch.Groups = []kmsg.OffsetFetchRequestGroup{group}
		response, e = client.Request(probe, fetch)
		if e == nil {
			r := response.(*kmsg.OffsetFetchResponse)
			for _, g := range r.Groups {
				for _, t := range g.Topics {
					for _, p := range t.Partitions {
						if p.ErrorCode == 0 {
							committed[p.Partition] = p.Offset
						}
					}
				}
			}
			for _, t := range r.Topics {
				for _, p := range t.Partitions {
					if p.ErrorCode == 0 {
						committed[p.Partition] = p.Offset
					}
				}
			}
		}
		if len(ends) == len(partitions) && offsetsCaughtUp(ends, committed) {
			return nil
		}
		if err = waitUntil(probe, time.Now().Add(50*time.Millisecond)); err != nil {
			return errors.New("consumer progress did not catch up before cleanup")
		}
	}
}
