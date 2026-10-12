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
				DescriptionKey: "Reply to tickets and change their status and priority.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})
}
