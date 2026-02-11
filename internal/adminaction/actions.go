package adminaction

type ActionType string

const (
	ActionMarkMilestonePaid              ActionType = "MARK_MILESTONE_PAID"
	ActionCompleteServiceWithoutFinalPay ActionType = "COMPLETE_SERVICE_WITHOUT_FINAL_PAYMENT"
	ActionReopenService                  ActionType = "REOPEN_SERVICE"
)


