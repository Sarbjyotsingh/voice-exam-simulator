package contracts

// AudioSlotKey represents canonical slot identifiers in pre-recorded script.json catalogues.
type AudioSlotKey string

const (
	// Examiner canonical slot keys
	SlotExaminerIntro            AudioSlotKey = "examiner_intro"
	SlotExaminerFollowUp1        AudioSlotKey = "examiner_followup_1"
	SlotExaminerFollowUp2        AudioSlotKey = "examiner_followup_2"
	SlotExaminerOutro            AudioSlotKey = "examiner_outro"
	SlotExaminerInterveneTopic   AudioSlotKey = "examiner_intervene_topic"
	SlotExaminerIntervenePolite  AudioSlotKey = "examiner_intervene_polite"
	SlotExaminerInterveneDialog  AudioSlotKey = "examiner_intervene_dialog"
	SlotExaminerInterveneSilence AudioSlotKey = "examiner_intervene_silence"

	// Partner canonical slot keys
	SlotPartnerOpening       AudioSlotKey = "partner_opening"
	SlotPartnerBackQ1        AudioSlotKey = "partner_backquestion_1"
	SlotPartnerBackQ2        AudioSlotKey = "partner_backquestion_2"
	SlotPartnerClarification AudioSlotKey = "partner_clarification"
	SlotPartnerSilence       AudioSlotKey = "partner_silence"
	SlotPartnerClosing       AudioSlotKey = "partner_closing"

	// Raw script.json slot codes used in DE_<SPEAKER>_<SLOT>_<KIND>
	CodeOrig   = "ORIG"
	CodeRes    = "RES"
	CodeFam    = "FAM"
	CodeWork   = "WORK"
	CodeLang   = "LANG"
	CodeGer    = "GER"
	CodeHob    = "HOB"
	CodeName   = "NAME"
	CodeAge    = "AGE"
	CodeDuSie  = "DUSIE"
	CodePres   = "PRES"
	CodeOpin   = "OPIN"
	CodeHab    = "HAB"
	CodeHome   = "HOME"
	CodePro    = "PRO"
	CodeTime   = "TIME"
	CodeLoc    = "LOC"
	CodeFood   = "FOOD"
	CodeTask   = "TASK"
	CodeSum    = "SUM"
	CodeBack   = "BACK"
	CodeT2Back = "T2BACK"
	CodeT3Back = "T3BACK"

	// Prefixes & suffixes
	PrefixExpression  = "expression_"
	PrefixExpCode     = "EXP_"
	SuffixQuestion1   = "Q1"
	SuffixQuestion2   = "Q2"
	SuffixSecondaryQ2 = "_q2"
	SuffixAnswer1     = "A1"
)
