package authz

const ResourceTicket = "ticket"

var (
	TicketRead  = Permission{Resource: ResourceTicket, Action: ActionRead}
	TicketWrite = Permission{Resource: ResourceTicket, Action: ActionWrite}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceTicket,
		LabelKey: "Ticket Management",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "View tickets",
				DescriptionKey: "View all users' support tickets and attachments.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Handle tickets",
				DescriptionKey: "Reply to tickets, change their status and priority, and clear the balance of refund ticket owners.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})
}
