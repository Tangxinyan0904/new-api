package authz

const ResourceChannelCost = "channel_cost"

// The cost statistics page shows business margins, so no role receives it by
// default; the root user grants it to individual administrators.
var (
	ChannelCostRead  = Permission{Resource: ResourceChannelCost, Action: ActionRead}
	ChannelCostWrite = Permission{Resource: ResourceChannelCost, Action: ActionWrite}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceChannelCost,
		LabelKey: "Cost Statistics",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "View cost statistics",
				DescriptionKey: "View channel usage, cost, and profit.",
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Edit cost ratios",
				DescriptionKey: "Change the default and per-channel cost ratios.",
			},
		},
	})
}
