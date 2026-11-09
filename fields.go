package dctl

const (
	maxOptionText              = 100
	componentStringSelect      = 3
	callbackUpdateMessage      = 7
	callbackChannelMessage     = 4
	callbackDeferredMessage    = 5
	callbackAutocompleteResult = 8
	maxAutocompleteChoices     = 25
	maxAttachments             = 10
	flagEphemeral              = 1 << 6
	fieldAttachments           = "attachments"
	fieldComponents            = "components"
	fieldContent               = "content"
	fieldData                  = "data"
	fieldName                  = "name"
	fieldType                  = "type"
	fieldValue                 = "value"
)
