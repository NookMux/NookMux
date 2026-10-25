package gemini

var ModelList = []string{
	"gemini-2.5-flash", "gemini-2.5-flash-image", "gemini-2.5-flash-lite",
	"gemini-2.5-pro", "gemini-2.5-pro-preview", "gemini-2.5-pro-preview-05-06",
	"gemini-3-flash-preview", "gemini-3-pro-image", "gemini-3-pro-image-preview",
	"gemini-3.1-flash-image", "gemini-3.1-flash-image-preview", "gemini-3.1-flash-lite",
	"gemini-3.1-flash-lite-image", "gemini-3.1-flash-lite-preview", "gemini-3.1-pro-preview",
	"gemini-3.1-pro-preview-customtools", "gemini-3.5-flash", "gemini-3.5-flash-lite",
	"gemini-3.6-flash", "gemini-3.8-flash", "gemini-4-argon",
	"gemini-embedding-001", "gemini-embedding-2", "gemini-nano-banana-2.1",
	"gemma-2-27b-it",
	"gemma-3-12b-it", "gemma-3-27b-it", "gemma-3-4b-it",
	"gemma-3n-e4b-it", "gemma-4-26b-a4b-it", "gemma-4-31b-it",
	"lyria-3-clip-preview", "lyria-3-pro-preview",
}

var SafetySettingList = []string{
	"HARM_CATEGORY_HARASSMENT",
	"HARM_CATEGORY_HATE_SPEECH",
	"HARM_CATEGORY_SEXUALLY_EXPLICIT",
	"HARM_CATEGORY_DANGEROUS_CONTENT",
	//"HARM_CATEGORY_CIVIC_INTEGRITY", This item is deprecated!
}

var ChannelName = "google gemini"
