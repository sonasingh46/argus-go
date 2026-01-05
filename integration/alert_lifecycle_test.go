package integration

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"argus-go/internal/domain"
	"argus-go/internal/notification"
	"argus-go/internal/processor"
	"argus-go/internal/queue"
	"argus-go/internal/queue/memory"
	"argus-go/internal/store"
	storemem "argus-go/internal/store/memory"
)

var _ = Describe("Alert Lifecycle", func() {
	var (
		processorService *processor.Service
		alertRepo        *storemem.AlertRepository
		eventManagerRepo *storemem.EventManagerRepository
		groupingRuleRepo *storemem.GroupingRuleRepository
		stateStore       *storemem.StateStore
		msgQueue         *memory.Queue
		ctx              context.Context
		cancel           context.CancelFunc
	)

	BeforeEach(func() {
		// Setup test context
		ctx, cancel = context.WithCancel(context.Background())

		// Initialize logger (quiet for tests)
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

		// Initialize in-memory stores
		stateStore = storemem.NewStateStore()
		alertRepo = storemem.NewAlertRepository()
		eventManagerRepo = storemem.NewEventManagerRepository()
		groupingRuleRepo = storemem.NewGroupingRuleRepository()

		// Initialize queue
		msgQueue = memory.NewQueue(1000)

		// Initialize notifier (stubbed)
		notifier := notification.NewStubNotifier(logger)

		// Initialize processor service
		processorService = processor.NewService(
			msgQueue,
			stateStore,
			alertRepo,
			eventManagerRepo,
			groupingRuleRepo,
			notifier,
			logger,
		)

		// Start processor in background
		go func() {
			_ = processorService.Start(ctx)
		}()

		// Give processor time to start
		time.Sleep(10 * time.Millisecond)
	})

	AfterEach(func() {
		cancel()
		_ = msgQueue.Close()
		stateStore.Clear()
		alertRepo.Clear()
		eventManagerRepo.Clear()
		groupingRuleRepo.Clear()
	})

	Describe("Event Manager and Grouping Rule Setup", func() {
		It("should create and retrieve an event manager", func() {
			// Create a grouping rule first
			rule := &domain.GroupingRule{
				ID:                "rule-1",
				Name:              "Test Rule",
				GroupingKey:       "class",
				TimeWindowMinutes: 5,
				CreatedAt:         time.Now(),
			}
			Expect(groupingRuleRepo.Create(ctx, rule)).To(Succeed())

			// Create event manager
			em := &domain.EventManager{
				ID:             "em-1",
				Name:           "Test EM",
				GroupingRuleID: "rule-1",
				CreatedAt:      time.Now(),
			}

			err := eventManagerRepo.Create(ctx, em)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := eventManagerRepo.GetByID(ctx, "em-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Name).To(Equal("Test EM"))
		})

		It("should create and retrieve a grouping rule", func() {
			rule := &domain.GroupingRule{
				ID:                "rule-1",
				Name:              "Database Alert Rule",
				GroupingKey:       "class",
				TimeWindowMinutes: 10,
				CreatedAt:         time.Now(),
			}

			err := groupingRuleRepo.Create(ctx, rule)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := groupingRuleRepo.GetByID(ctx, "rule-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Name).To(Equal("Database Alert Rule"))
			Expect(retrieved.TimeWindowMinutes).To(Equal(10))
		})
	})

	Describe("Alert Grouping", func() {
		BeforeEach(func() {
			// Setup grouping rule
			rule := &domain.GroupingRule{
				ID:                "rule-1",
				Name:              "Test Rule",
				GroupingKey:       "class",
				TimeWindowMinutes: 5,
				CreatedAt:         time.Now(),
			}
			Expect(groupingRuleRepo.Create(ctx, rule)).To(Succeed())

			// Setup event manager
			em := &domain.EventManager{
				ID:             "em-1",
				Name:           "Test EM",
				GroupingRuleID: "rule-1",
				CreatedAt:      time.Now(),
			}
			Expect(eventManagerRepo.Create(ctx, em)).To(Succeed())
		})

		It("should create a parent alert for the first event", func() {
			event := &domain.InternalEvent{
				Event: domain.Event{
					EventManagerID: "em-1",
					Summary:        "Database connection failed",
					Severity:       domain.SeverityHigh,
					Action:         domain.ActionTrigger,
					Class:          "database",
					DedupKey:       "db-alert-1",
				},
				PartitionKey:  "partition-1",
				GroupingValue: "database",
				ReceivedAt:    time.Now(),
			}

			// Publish directly to queue (simulating ingest service)
			payload, _ := json.Marshal(event)
			err := msgQueue.Publish(ctx, &queue.Message{
				Key:   []byte(event.PartitionKey),
				Value: payload,
			})
			Expect(err).NotTo(HaveOccurred())

			// Wait for processing
			Eventually(func() int {
				alerts, _ := alertRepo.List(ctx, domain.AlertFilter{})
				return len(alerts)
			}, 1*time.Second, 10*time.Millisecond).Should(Equal(1))

			// Verify alert
			alert, err := alertRepo.GetByDedupKey(ctx, "db-alert-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(alert.Type).To(Equal(domain.AlertTypeParent))
			Expect(alert.Status).To(Equal(domain.AlertStatusActive))
		})

		It("should group subsequent events as children", func() {
			// Create parent event
			parentEvent := &domain.InternalEvent{
				Event: domain.Event{
					EventManagerID: "em-1",
					Summary:        "Parent alert",
					Severity:       domain.SeverityHigh,
					Action:         domain.ActionTrigger,
					Class:          "database",
					DedupKey:       "parent-1",
				},
				PartitionKey:  "partition-1",
				GroupingValue: "database",
				ReceivedAt:    time.Now(),
			}

			payload, _ := json.Marshal(parentEvent)
			_ = msgQueue.Publish(ctx, &queue.Message{Key: []byte("p1"), Value: payload})

			// Wait for parent to be created
			Eventually(func() bool {
				alert, _ := alertRepo.GetByDedupKey(ctx, "parent-1")
				return alert != nil
			}, 1*time.Second).Should(BeTrue())

			// Create child event
			childEvent := &domain.InternalEvent{
				Event: domain.Event{
					EventManagerID: "em-1",
					Summary:        "Child alert",
					Severity:       domain.SeverityMedium,
					Action:         domain.ActionTrigger,
					Class:          "database", // Same class = same group
					DedupKey:       "child-1",
				},
				PartitionKey:  "partition-1",
				GroupingValue: "database",
				ReceivedAt:    time.Now(),
			}

			payload, _ = json.Marshal(childEvent)
			_ = msgQueue.Publish(ctx, &queue.Message{Key: []byte("p1"), Value: payload})

			// Wait for child to be created
			Eventually(func() bool {
				alert, _ := alertRepo.GetByDedupKey(ctx, "child-1")
				return alert != nil
			}, 1*time.Second).Should(BeTrue())

			// Verify child
			child, _ := alertRepo.GetByDedupKey(ctx, "child-1")
			Expect(child.Type).To(Equal(domain.AlertTypeChild))
			Expect(child.ParentDedupKey).To(Equal("parent-1"))
		})

		It("should not group events with different grouping values", func() {
			// Create first event with class "database"
			event1 := &domain.InternalEvent{
				Event: domain.Event{
					EventManagerID: "em-1",
					Summary:        "Database alert",
					Severity:       domain.SeverityHigh,
					Action:         domain.ActionTrigger,
					Class:          "database",
					DedupKey:       "db-alert-1",
				},
				PartitionKey:  "partition-1",
				GroupingValue: "database",
				ReceivedAt:    time.Now(),
			}

			payload, _ := json.Marshal(event1)
			_ = msgQueue.Publish(ctx, &queue.Message{Key: []byte("p1"), Value: payload})

			// Wait for first alert
			Eventually(func() bool {
				alert, _ := alertRepo.GetByDedupKey(ctx, "db-alert-1")
				return alert != nil
			}, 1*time.Second).Should(BeTrue())

			// Create second event with different class
			event2 := &domain.InternalEvent{
				Event: domain.Event{
					EventManagerID: "em-1",
					Summary:        "Web alert",
					Severity:       domain.SeverityHigh,
					Action:         domain.ActionTrigger,
					Class:          "web", // Different class = different group
					DedupKey:       "web-alert-1",
				},
				PartitionKey:  "partition-2",
				GroupingValue: "web",
				ReceivedAt:    time.Now(),
			}

			payload, _ = json.Marshal(event2)
			_ = msgQueue.Publish(ctx, &queue.Message{Key: []byte("p2"), Value: payload})

			// Wait for second alert
			Eventually(func() bool {
				alert, _ := alertRepo.GetByDedupKey(ctx, "web-alert-1")
				return alert != nil
			}, 1*time.Second).Should(BeTrue())

			// Both should be parent alerts
			dbAlert, _ := alertRepo.GetByDedupKey(ctx, "db-alert-1")
			Expect(dbAlert.Type).To(Equal(domain.AlertTypeParent))

			webAlert, _ := alertRepo.GetByDedupKey(ctx, "web-alert-1")
			Expect(webAlert.Type).To(Equal(domain.AlertTypeParent))
		})
	})

	Describe("Alert Resolution", func() {
		BeforeEach(func() {
			// Setup grouping rule
			rule := &domain.GroupingRule{
				ID:                "rule-1",
				Name:              "Test Rule",
				GroupingKey:       "class",
				TimeWindowMinutes: 5,
				CreatedAt:         time.Now(),
			}
			Expect(groupingRuleRepo.Create(ctx, rule)).To(Succeed())

			// Setup event manager
			em := &domain.EventManager{
				ID:             "em-1",
				Name:           "Test EM",
				GroupingRuleID: "rule-1",
				CreatedAt:      time.Now(),
			}
			Expect(eventManagerRepo.Create(ctx, em)).To(Succeed())
		})

		It("should resolve a child alert independently", func() {
			// Create parent and child manually
			parent := &domain.Alert{
				ID:             "parent-id",
				DedupKey:       "parent-1",
				EventManagerID: "em-1",
				Type:           domain.AlertTypeParent,
				Status:         domain.AlertStatusActive,
				ChildCount:     1,
				CreatedAt:      time.Now(),
			}
			_ = alertRepo.Create(ctx, parent)
			_ = stateStore.SetAlert(ctx, &store.AlertState{
				DedupKey:       "parent-1",
				EventManagerID: "em-1",
				Type:           "parent",
				Status:         "active",
			})

			child := &domain.Alert{
				ID:             "child-id",
				DedupKey:       "child-1",
				EventManagerID: "em-1",
				Type:           domain.AlertTypeChild,
				Status:         domain.AlertStatusActive,
				ParentDedupKey: "parent-1",
				CreatedAt:      time.Now(),
			}
			_ = alertRepo.Create(ctx, child)
			_ = stateStore.SetAlert(ctx, &store.AlertState{
				DedupKey:       "child-1",
				EventManagerID: "em-1",
				Type:           "child",
				Status:         "active",
				ParentDedupKey: "parent-1",
			})

			// Send resolve event for child
			resolveEvent := &domain.InternalEvent{
				Event: domain.Event{
					EventManagerID: "em-1",
					Action:         domain.ActionResolve,
					DedupKey:       "child-1",
				},
				ReceivedAt: time.Now(),
			}

			payload, _ := json.Marshal(resolveEvent)
			_ = msgQueue.Publish(ctx, &queue.Message{Value: payload})

			// Wait for resolution
			Eventually(func() domain.AlertStatus {
				alert, _ := alertRepo.GetByDedupKey(ctx, "child-1")
				if alert == nil {
					return ""
				}
				return alert.Status
			}, 1*time.Second).Should(Equal(domain.AlertStatusResolved))

			// Parent should still be active
			parentAlert, _ := alertRepo.GetByDedupKey(ctx, "parent-1")
			Expect(parentAlert.Status).To(Equal(domain.AlertStatusActive))
		})

		It("should resolve parent only after all children are resolved", func() {
			// Create parent with pending resolve
			parent := &domain.Alert{
				ID:               "parent-id",
				DedupKey:         "parent-1",
				EventManagerID:   "em-1",
				Type:             domain.AlertTypeParent,
				Status:           domain.AlertStatusActive,
				ChildCount:       1,
				ResolveRequested: true,
				CreatedAt:        time.Now(),
			}
			_ = alertRepo.Create(ctx, parent)
			_ = stateStore.SetAlert(ctx, &store.AlertState{
				DedupKey:         "parent-1",
				EventManagerID:   "em-1",
				Type:             "parent",
				Status:           "active",
				ResolveRequested: true,
			})
			_ = stateStore.SetPendingResolve(ctx, "parent-1", &store.PendingResolve{
				RequestedAt:       time.Now(),
				RemainingChildren: 1,
			})

			// Create active child
			child := &domain.Alert{
				ID:             "child-id",
				DedupKey:       "child-1",
				EventManagerID: "em-1",
				Type:           domain.AlertTypeChild,
				Status:         domain.AlertStatusActive,
				ParentDedupKey: "parent-1",
				CreatedAt:      time.Now(),
			}
			_ = alertRepo.Create(ctx, child)
			_ = stateStore.SetAlert(ctx, &store.AlertState{
				DedupKey:       "child-1",
				EventManagerID: "em-1",
				Type:           "child",
				Status:         "active",
				ParentDedupKey: "parent-1",
			})
			_ = stateStore.AddChild(ctx, "parent-1", "child-1")

			// Resolve child
			resolveEvent := &domain.InternalEvent{
				Event: domain.Event{
					EventManagerID: "em-1",
					Action:         domain.ActionResolve,
					DedupKey:       "child-1",
				},
				ReceivedAt: time.Now(),
			}

			payload, _ := json.Marshal(resolveEvent)
			_ = msgQueue.Publish(ctx, &queue.Message{Value: payload})

			// Both should be resolved now
			Eventually(func() domain.AlertStatus {
				alert, _ := alertRepo.GetByDedupKey(ctx, "parent-1")
				if alert == nil {
					return ""
				}
				return alert.Status
			}, 1*time.Second).Should(Equal(domain.AlertStatusResolved))
		})

		It("should defer parent resolution when children are still active", func() {
			// Create parent and active child
			parent := &domain.Alert{
				ID:             "parent-id",
				DedupKey:       "parent-1",
				EventManagerID: "em-1",
				Type:           domain.AlertTypeParent,
				Status:         domain.AlertStatusActive,
				ChildCount:     1,
				CreatedAt:      time.Now(),
			}
			_ = alertRepo.Create(ctx, parent)
			_ = stateStore.SetAlert(ctx, &store.AlertState{
				DedupKey:       "parent-1",
				EventManagerID: "em-1",
				Type:           "parent",
				Status:         "active",
			})

			child := &domain.Alert{
				ID:             "child-id",
				DedupKey:       "child-1",
				EventManagerID: "em-1",
				Type:           domain.AlertTypeChild,
				Status:         domain.AlertStatusActive,
				ParentDedupKey: "parent-1",
				CreatedAt:      time.Now(),
			}
			_ = alertRepo.Create(ctx, child)
			_ = stateStore.AddChild(ctx, "parent-1", "child-1")

			// Try to resolve parent (should be deferred)
			resolveEvent := &domain.InternalEvent{
				Event: domain.Event{
					EventManagerID: "em-1",
					Action:         domain.ActionResolve,
					DedupKey:       "parent-1",
				},
				ReceivedAt: time.Now(),
			}

			payload, _ := json.Marshal(resolveEvent)
			_ = msgQueue.Publish(ctx, &queue.Message{Value: payload})

			// Parent should have ResolveRequested=true but still be active
			Eventually(func() bool {
				alert, _ := alertRepo.GetByDedupKey(ctx, "parent-1")
				if alert == nil {
					return false
				}
				return alert.ResolveRequested
			}, 1*time.Second).Should(BeTrue())

			parentAlert, _ := alertRepo.GetByDedupKey(ctx, "parent-1")
			Expect(parentAlert.Status).To(Equal(domain.AlertStatusActive))
		})
	})

	Describe("Deduplication", func() {
		BeforeEach(func() {
			// Setup grouping rule
			rule := &domain.GroupingRule{
				ID:                "rule-1",
				Name:              "Test Rule",
				GroupingKey:       "class",
				TimeWindowMinutes: 5,
				CreatedAt:         time.Now(),
			}
			Expect(groupingRuleRepo.Create(ctx, rule)).To(Succeed())

			// Setup event manager
			em := &domain.EventManager{
				ID:             "em-1",
				Name:           "Test EM",
				GroupingRuleID: "rule-1",
				CreatedAt:      time.Now(),
			}
			Expect(eventManagerRepo.Create(ctx, em)).To(Succeed())
		})

		It("should not create duplicate alerts for same dedupKey", func() {
			// Create first event
			event1 := &domain.InternalEvent{
				Event: domain.Event{
					EventManagerID: "em-1",
					Summary:        "First alert",
					Severity:       domain.SeverityHigh,
					Action:         domain.ActionTrigger,
					Class:          "database",
					DedupKey:       "alert-1",
				},
				PartitionKey:  "partition-1",
				GroupingValue: "database",
				ReceivedAt:    time.Now(),
			}

			payload, _ := json.Marshal(event1)
			_ = msgQueue.Publish(ctx, &queue.Message{Key: []byte("p1"), Value: payload})

			// Wait for first alert
			Eventually(func() bool {
				alert, _ := alertRepo.GetByDedupKey(ctx, "alert-1")
				return alert != nil
			}, 1*time.Second).Should(BeTrue())

			// Send duplicate event with same dedupKey
			event2 := &domain.InternalEvent{
				Event: domain.Event{
					EventManagerID: "em-1",
					Summary:        "Duplicate alert",
					Severity:       domain.SeverityHigh,
					Action:         domain.ActionTrigger,
					Class:          "database",
					DedupKey:       "alert-1", // Same dedupKey
				},
				PartitionKey:  "partition-1",
				GroupingValue: "database",
				ReceivedAt:    time.Now(),
			}

			payload, _ = json.Marshal(event2)
			_ = msgQueue.Publish(ctx, &queue.Message{Key: []byte("p1"), Value: payload})

			// Wait a bit for processing
			time.Sleep(100 * time.Millisecond)

			// Should still have only 1 alert
			alerts, _ := alertRepo.List(ctx, domain.AlertFilter{})
			Expect(len(alerts)).To(Equal(1))

			// Summary should be from first event
			alert, _ := alertRepo.GetByDedupKey(ctx, "alert-1")
			Expect(alert.Summary).To(Equal("First alert"))
		})
	})
})
