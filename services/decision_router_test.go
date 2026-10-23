package services

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"voice-exam-simulator/config"
)

func TestParseScriptCatalogueAll5Samples(t *testing.T) {
	samples := []struct {
		folder    string
		primary   string
		secondary string
		expPrefix string
	}{
		{"B1_Exam_Sample_01", "Leyla", "Herr Wagner", "DE_LEY_"},
		{"B1_Exam_Sample_02", "Omar", "Frau Fischer", "DE_OMA_"},
		{"B1_Exam_Sample_03", "Elena", "Frau Fischer", "DE_ELE_"},
		{"B1_Exam_Sample_04", "Mateo", "Herr Richter", "DE_MAT_"},
		{"B1_Exam_Sample_05", "Janusz", "Frau Dr. Meyer", "DE_JAN_"},
	}

	for _, s := range samples {
		p := filepath.Join("..", "exam_generator", "output_s3", s.folder, "Teil_1", "script.json")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("could not read %s: %v", p, err)
		}

		var scriptData struct {
			Dialogues []map[string]interface{} `json:"dialogues"`
		}
		if err := json.Unmarshal(b, &scriptData); err != nil {
			t.Fatalf("failed to decode %s: %v", p, err)
		}
		dialoguesJSON, _ := json.Marshal(scriptData.Dialogues)
		wrappedPrompt := "AVAILABLE PRE-RECORDED AUDIO SCRIPT:\n" + string(dialoguesJSON) + "\n\n### 1. PERSONAS\n..."

		cat, err := ParseScriptCatalogue(wrappedPrompt, s.primary, s.secondary)
		if err != nil {
			t.Fatalf("[%s] ParseScriptCatalogue failed: %v", s.folder, err)
		}

		if len(cat.ClipsByID) < 30 {
			t.Errorf("[%s] expected >= 30 clips, got %d", s.folder, len(cat.ClipsByID))
		}
		if len(cat.PartnerExp) < 18 {
			t.Errorf("[%s] expected 18 EXP clips, got %d", s.folder, len(cat.PartnerExp))
		}
		for _, required := range []string{"ORIG", "RES", "FAM", "WORK", "LANG", "GER", "HOB"} {
			if _, ok := cat.PartnerQuestions[required]; !ok {
				t.Errorf("[%s] missing PartnerQuestion for %s", s.folder, required)
			}
			if _, ok := cat.PartnerAnswers[required]; !ok {
				t.Errorf("[%s] missing PartnerAnswer for %s", s.folder, required)
			}
		}
		for _, required := range []string{"INT_TOPIC", "INT_POLITE", "OUTRO_01", "FOLLOW_01"} {
			if _, ok := cat.ExaminerClips[required]; !ok {
				t.Errorf("[%s] missing ExaminerClip for %s", s.folder, required)
			}
		}
	}
}

func TestParseScriptCatalogueTeil2All16Samples(t *testing.T) {
	samples := []struct {
		folder    string
		primary   string
		secondary string
		expPrefix string
	}{
		{"B1_Exam_Sample_01", "Leyla", "Herr Wagner", "DE_LEY_"},
		{"B1_Exam_Sample_02", "Omar", "Frau Fischer", "DE_OMA_"},
		{"B1_Exam_Sample_03", "Elena", "Frau Erdoğan", "DE_ELE_"},
		{"B1_Exam_Sample_04", "Mateo", "Herr Richter", "DE_MAT_"},
		{"B1_Exam_Sample_05", "Janusz", "Frau Dr. Meyer", "DE_JAN_"},
		{"B1_Exam_Sample_06", "Leyla", "Herr Wagner", "DE_LEY_"},
		{"B1_Exam_Sample_07", "Omar", "Frau Fischer", "DE_OMA_"},
		{"B1_Exam_Sample_08", "Elena", "Frau Erdoğan", "DE_ELE_"},
		{"B1_Exam_Sample_09", "Leyla", "Herr Wagner", "DE_LEY_"},
		{"B1_Exam_Sample_10", "Omar", "Frau Fischer", "DE_OMA_"},
		{"B1_Exam_Sample_11", "Elena", "Frau Erdoğan", "DE_ELE_"},
		{"B1_Exam_Sample_12", "Mateo", "Herr Richter", "DE_MAT_"},
		{"B1_Exam_Sample_13", "Janusz", "Frau Dr. Meyer", "DE_JAN_"},
		{"B1_Exam_Sample_14", "Leyla", "Herr Wagner", "DE_LEY_"},
		{"B1_Exam_Sample_15", "Omar", "Frau Fischer", "DE_OMA_"},
		{"B1_Exam_Sample_16", "Elena", "Frau Erdoğan", "DE_ELE_"},
	}

	for _, s := range samples {
		p := filepath.Join("..", "exam_generator", "output_s3", s.folder, "Teil_2", "script.json")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("could not read %s: %v", p, err)
		}

		var scriptData struct {
			Dialogues []map[string]interface{} `json:"dialogues"`
		}
		if err := json.Unmarshal(b, &scriptData); err != nil {
			t.Fatalf("failed to decode %s: %v", p, err)
		}
		dialoguesJSON, _ := json.Marshal(scriptData.Dialogues)
		wrappedPrompt := "AVAILABLE PRE-RECORDED AUDIO SCRIPT:\n" + string(dialoguesJSON) + "\n\n### 1. PERSONAS\n..."

		cat, err := ParseScriptCatalogue(wrappedPrompt, s.primary, s.secondary)
		if err != nil {
			t.Fatalf("[%s/Teil_2] ParseScriptCatalogue failed: %v", s.folder, err)
		}

		if len(cat.ClipsByID) != 44 {
			t.Errorf("[%s/Teil_2] expected 44 clips, got %d", s.folder, len(cat.ClipsByID))
		}
		if len(cat.PartnerExp) != 18 {
			t.Errorf("[%s/Teil_2] expected 18 EXP clips, got %d", s.folder, len(cat.PartnerExp))
		}
		for _, required := range []string{"PRES", "OPIN", "HAB", "HOME", "PRO"} {
			if _, ok := cat.PartnerQuestions[required]; !ok {
				t.Errorf("[%s/Teil_2] missing PartnerQuestion for %s", s.folder, required)
			}
			if _, ok := cat.PartnerAnswers[required]; !ok {
				t.Errorf("[%s/Teil_2] missing PartnerAnswer for %s", s.folder, required)
			}
		}
		for _, required := range []string{"T2_INT_TOPIC", "T2_INT_POLITE", "T2_INT_DIALOG", "T2_OUTRO_01", "T2_FOLLOW_01"} {
			if _, ok := cat.ExaminerClips[required]; !ok {
				t.Errorf("[%s/Teil_2] missing ExaminerClip for %s", s.folder, required)
			}
		}
	}
}

func TestDecisionRouterTeil2DeterministicComposition(t *testing.T) {
	p := filepath.Join("..", "exam_generator", "output_s3", "B1_Exam_Sample_01", "Teil_2", "script.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("could not read %s: %v", p, err)
	}
	var scriptData struct {
		Dialogues []map[string]interface{} `json:"dialogues"`
	}
	_ = json.Unmarshal(b, &scriptData)
	dialoguesJSON, _ := json.Marshal(scriptData.Dialogues)
	wrappedPrompt := "AVAILABLE PRE-RECORDED AUDIO SCRIPT:\n" + string(dialoguesJSON) + "\n\n### 1. PERSONAS\n..."

	cat, err := ParseScriptCatalogue(wrappedPrompt, "Leyla", "Herr Wagner")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	spec := DefaultB1Teil2Spec()
	router := &DecisionRouter{}

	// Turn 1: Candidate summarizes Hannes Leivig -> Partner must react with EXP_36, share OPIN_A1, and ask OPIN_Q1
	st1 := ConversationState{CoveredTopics: map[string]bool{"presentation": true}, TurnCount: 1}
	ans1 := map[string]DecisionAnswer{
		"topic_addressed":       {Choice: "presentation"},
		"reaction_clip":         {Choice: "EXP_21"},
		"learner_asked_partner": {Probability: floatPtr(0.0)},
	}
	res1, err := router.composeNormalTurn(spec, cat, st1, ans1)
	if err != nil || res1.Fallback || len(res1.Parts) != 3 {
		t.Fatalf("expected 3 parts on Teil 2 Turn 1 presentation, got %+v", res1.Parts)
	}
	if res1.Parts[0]["audio_id"] != "DE_LEY_EXP_36" {
		t.Errorf("expected DE_LEY_EXP_36, got %v", res1.Parts[0]["audio_id"])
	}
	if res1.Parts[1]["audio_id"] != "DE_LEYT1A_OPIN_A1" {
		t.Errorf("expected DE_LEYT1A_OPIN_A1, got %v", res1.Parts[1]["audio_id"])
	}
	if res1.Parts[2]["audio_id"] != "DE_LEYT1_OPIN_Q1" {
		t.Errorf("expected DE_LEYT1_OPIN_Q1, got %v", res1.Parts[2]["audio_id"])
	}

	// Turn 1 Premature-Discussion Guard: if learner skips text summary and gives personal opinion on Turn 1 -> [DE_LEY_EXP_27, DE_LEY_PRES_Q2]
	ansPremature := map[string]DecisionAnswer{
		"topic_addressed":       {Choice: "opinion"},
		"reaction_clip":         {Choice: "EXP_21"},
		"learner_asked_partner": {Probability: floatPtr(0.0)},
	}
	resPremature, err := router.composeNormalTurn(spec, cat, st1, ansPremature)
	if err != nil || resPremature.Fallback || len(resPremature.Parts) != 2 {
		t.Fatalf("expected 2 parts on Teil 2 Turn 1 premature discussion, got %+v", resPremature.Parts)
	}
	if resPremature.Parts[0]["audio_id"] != "DE_LEY_EXP_27" || resPremature.Parts[1]["audio_id"] != "DE_LEY_PRES_Q2" {
		t.Errorf("expected [DE_LEY_EXP_27, DE_LEY_PRES_Q2], got %+v", resPremature.Parts)
	}

	// Turn 5: Once all 5 required slots (presentation, opinion, habits_germany, home_country, pros_cons) are covered -> T2CLOSE_01 + T2_FOLLOW_01
	stAll5 := ConversationState{
		CoveredTopics: map[string]bool{
			"presentation": true, "opinion": true, "habits_germany": true,
			"home_country": true, "pros_cons": true,
		},
		TurnCount: 5,
	}
	ansWrap := map[string]DecisionAnswer{
		"topic_addressed":       {Choice: "pros_cons"},
		"reaction_clip":         {Choice: "EXP_25"},
		"learner_asked_partner": {Probability: floatPtr(0.0)},
	}
	resWrap, _ := router.composeNormalTurn(spec, cat, stAll5, ansWrap)
	if len(resWrap.Parts) != 3 || resWrap.Parts[1]["audio_id"] != "DE_LEY_T2CLOSE_01" || resWrap.Parts[2]["audio_id"] != "DE_EX1T1_T2_FOLLOW_01" {
		t.Fatalf("expected [DE_LEY_EXP_25, DE_LEY_T2CLOSE_01, DE_EX1T1_T2_FOLLOW_01] on Teil 2 wrap-up, got %+v", resWrap.Parts)
	}
}

func TestDecisionRouterDeterministicComposition(t *testing.T) {
	p := filepath.Join("..", "exam_generator", "output_s3", "B1_Exam_Sample_01", "Teil_1", "script.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("could not read %s: %v", p, err)
	}
	var scriptData struct {
		Dialogues []map[string]interface{} `json:"dialogues"`
	}
	_ = json.Unmarshal(b, &scriptData)
	dialoguesJSON, _ := json.Marshal(scriptData.Dialogues)
	wrappedPrompt := "AVAILABLE PRE-RECORDED AUDIO SCRIPT:\n" + string(dialoguesJSON) + "\n\n### 1. PERSONAS\n..."

	cat, err := ParseScriptCatalogue(wrappedPrompt, "Leyla", "Herr Wagner")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	spec := DefaultB1Teil1Spec()
	router := &DecisionRouter{}

	// Case 0: AnalyzeHistory MUST ignore the system prompt (which embeds all script clips)
	historyWithSystem := []Message{
		{Role: "system", Content: wrappedPrompt},
		{Role: "assistant", Content: "[Herr Wagner]: Guten Tag. Herzlich willkommen zur mündlichen telc B1 Prüfung."},
		{Role: "assistant", Content: "[Leyla]: Hallo! Wollen wir du sagen? Ich fange einfach mal an: Ich heiße Leyla Yılmaz. Und wie heißt du?"},
		{Role: "user", Content: "Mein Name ist Sarabjot Singh Chahal, ich komme aus Indien."},
	}
	stInit := AnalyzeHistory(historyWithSystem, cat, spec)
	if stInit.AwaitingFollowUpAnswer {
		t.Fatalf("AnalyzeHistory falsely detected AwaitingFollowUpAnswer=true from system prompt!")
	}
	if len(stInit.CoveredTopics) != 1 || !stInit.CoveredTopics["name"] {
		t.Fatalf("AnalyzeHistory falsely marked topics covered from system prompt: %+v", stInit.CoveredTopics)
	}
	if len(stInit.UsedExpressions) != 0 {
		t.Fatalf("AnalyzeHistory falsely marked expressions used from system prompt: %+v", stInit.UsedExpressions)
	}

	// Case 1: Polite intervention on rude input
	st := ConversationState{CoveredTopics: map[string]bool{"name": true}, LastAskedTopic: "ORIG"}
	res, err := router.composeIntervention(spec.Interventions.PolitenessSlot, cat, st)
	if err != nil || res.Fallback {
		t.Fatalf("unexpected fallback on intervention: %v", err)
	}
	if len(res.Parts) != 2 || res.Parts[0]["audio_id"] != "DE_EX1_INT_POLITE" || res.Parts[1]["audio_id"] != "DE_LEY_ORIG_Q1" {
		t.Errorf("unexpected intervention parts: %+v", res.Parts)
	}

	// Case 2: Turn 1 Name - MUST use EXP_11 and NOT play Leyla's name answer
	st1 := ConversationState{CoveredTopics: map[string]bool{"name": true}, TurnCount: 1}
	ans1 := map[string]DecisionAnswer{
		"topic_addressed":       {Choice: "name"},
		"reaction_clip":         {Choice: "EXP_04"},
		"learner_asked_partner": {Probability: floatPtr(0.0)},
	}
	res1, _ := router.composeNormalTurn(spec, cat, st1, ans1)
	if res1.Fallback || len(res1.Parts) < 2 {
		t.Fatalf("unexpected Turn 1 result: %+v", res1)
	}
	if res1.Parts[0]["audio_id"] != "DE_LEY_EXP_11" {
		t.Errorf("Turn 1 reaction expected DE_LEY_EXP_11, got %v", res1.Parts[0]["audio_id"])
	}
	for _, p := range res1.Parts {
		if p["audio_id"] == "DE_LEY_NAME_A1" || p["audio_id"] == "DE_LEY_DUSIE_A1" {
			t.Errorf("partner played forbidden self-answer on Turn 1: %v", p["audio_id"])
		}
	}

	// Case 3: Counter-question asked on Origin - MUST play Leyla's ORIG_A1 and next question (RES_Q1)
	st2 := ConversationState{CoveredTopics: map[string]bool{"name": true}, TurnCount: 2}
	ans2 := map[string]DecisionAnswer{
		"topic_addressed":       {Choice: "origin"},
		"reaction_clip":         {Choice: "EXP_02"},
		"learner_asked_partner": {Probability: floatPtr(1.0)},
	}
	res2, _ := router.composeNormalTurn(spec, cat, st2, ans2)
	if res2.Fallback || len(res2.Parts) != 3 {
		t.Fatalf("expected 3 parts (EXP, Answer, NextQ), got %+v", res2.Parts)
	}
	if res2.Parts[0]["audio_id"] != "DE_LEY_EXP_02" {
		t.Errorf("expected EXP_02, got %v", res2.Parts[0]["audio_id"])
	}
	if res2.Parts[1]["audio_id"] != "DE_LEY_ORIG_A1" {
		t.Errorf("expected ORIG_A1, got %v", res2.Parts[1]["audio_id"])
	}
	if res2.Parts[2]["audio_id"] != "DE_LEY_RES_Q1" {
		t.Errorf("expected RES_Q1, got %v", res2.Parts[2]["audio_id"])
	}

	// Case 4: Learner answers family AND asks partner about work ("Ich habe eine Familie... Und was machst du beruflich?")
	// Partner must answer WORK_A1 (not FAM_A1) and still ask WORK_Q1!
	stCross := ConversationState{
		CoveredTopics: map[string]bool{"name": true, "origin": true, "residence": true, "family": true},
		TurnCount:     3,
	}
	ansCross := map[string]DecisionAnswer{
		"topic_addressed":        {Choice: "family"},
		"partner_question_topic": {Choice: "work"},
		"reaction_clip":          {Choice: "EXP_02"},
		"learner_asked_partner":  {Probability: floatPtr(1.0)},
	}
	resCross, _ := router.composeNormalTurn(spec, cat, stCross, ansCross)
	if len(resCross.Parts) != 3 {
		t.Fatalf("expected 3 parts on cross-topic counter-question, got %+v", resCross.Parts)
	}
	if resCross.Parts[1]["audio_id"] != "DE_LEY_WORK_A1" {
		t.Errorf("expected DE_LEY_WORK_A1 for partner_question_topic=work, got %v", resCross.Parts[1]["audio_id"])
	}
	if resCross.Parts[2]["audio_id"] != "DE_LEY_WORK_Q1" {
		t.Errorf("expected DE_LEY_WORK_Q1 to still be asked to learner, got %v", resCross.Parts[2]["audio_id"])
	}

	// Case 5: Full multi-turn progression must ask GER_Q1 and HOB_Q1 before wrapping up with CLOSE_01 + FOLLOW_01
	stBeforeHobbies := ConversationState{
		CoveredTopics: map[string]bool{
			"name": true, "origin": true, "residence": true,
			"family": true, "work": true, "languages": true, "german_learning": true,
		},
		TurnCount: 6,
	}
	ansBeforeHobbies := map[string]DecisionAnswer{
		"topic_addressed":       {Choice: "german_learning"},
		"reaction_clip":         {Choice: "EXP_08"},
		"learner_asked_partner": {Probability: floatPtr(0.0)},
	}
	resHobQ, _ := router.composeNormalTurn(spec, cat, stBeforeHobbies, ansBeforeHobbies)
	if len(resHobQ.Parts) != 2 || resHobQ.Parts[1]["audio_id"] != "DE_LEY_HOB_Q1" {
		t.Fatalf("expected DE_LEY_HOB_Q1 on turn 6 (7/8 topics covered), got %+v", resHobQ.Parts)
	}

	// Only AFTER hobbies has also been asked (8/8 required topics covered) should it play CLOSE_01 + FOLLOW_01
	stAll8Covered := ConversationState{
		CoveredTopics: map[string]bool{
			"name": true, "origin": true, "residence": true,
			"family": true, "work": true, "languages": true,
			"german_learning": true, "hobbies": true,
		},
		TurnCount: 7,
	}
	ansAfterHobbies := map[string]DecisionAnswer{
		"topic_addressed":       {Choice: "hobbies"},
		"reaction_clip":         {Choice: "EXP_09"},
		"learner_asked_partner": {Probability: floatPtr(0.0)},
	}
	resWrap, _ := router.composeNormalTurn(spec, cat, stAll8Covered, ansAfterHobbies)
	if len(resWrap.Parts) != 3 || resWrap.Parts[1]["audio_id"] != "DE_LEY_CLOSE_01" || resWrap.Parts[2]["audio_id"] != "DE_EX1_FOLLOW_01" {
		t.Fatalf("expected [EXP_09, DE_LEY_CLOSE_01, DE_EX1_FOLLOW_01] after all 8 topics covered, got %+v", resWrap.Parts)
	}
}

func TestPricingWithDecisions(t *testing.T) {
	cfg := config.Config{
		WhisperPricePerHourUSD:  0.111,
		GroqInputPer1MUSD:       0.15,
		GroqOutputPer1MUSD:      0.60,
		DecisionsInputPer1MUSD:  0.10,
		ElevenLabsPer1KCharsUSD: 0.05,
	}
	p := NewPricing(cfg)

	// Turn 1: STT (5s -> billed 10s min) + Decisions (1,450 tokens) + 0 Groq + 0 TTS
	u1 := UsageSnapshot{
		STTSeconds:     5.0,
		DecisionTokens: 1450,
	}
	c1 := p.TurnCost(u1)
	expectedSTT := 10.0 / 3600.0 * 0.111 // ~0.0003083
	expectedDec := 1450.0 / 1e6 * 0.10    // ~0.000145
	if c1.Decisions == 0 || c1.Decisions != expectedDec {
		t.Errorf("expected decisions cost %f, got %f", expectedDec, c1.Decisions)
	}
	if c1.Total != c1.STT+c1.Decisions {
		t.Errorf("expected total %f, got %f", c1.STT+c1.Decisions, c1.Total)
	}

	// Turn 2: Fallback turn (STT + Decisions + Groq + ElevenLabs TTS)
	u2 := UsageSnapshot{
		STTSeconds:       6.0,
		DecisionTokens:   1450,
		PromptTokens:     2000,
		CompletionTokens: 300,
		TTSCalls: []TTSCall{
			{VoiceID: "some_voice", Characters: 120},
		},
	}
	c2 := p.TurnCost(u2)
	expectedGroq := 2000.0/1e6*0.15 + 300.0/1e6*0.60
	expectedTTS := 120.0 / 1000.0 * 0.05
	if math.Abs(c2.ChatLLM-expectedGroq) > 1e-9 {
		t.Errorf("expected chat LLM %f, got %f", expectedGroq, c2.ChatLLM)
	}
	if math.Abs(c2.TTS-expectedTTS) > 1e-9 {
		t.Errorf("expected TTS %f, got %f", expectedTTS, c2.TTS)
	}
	if math.Abs(c2.Total-(c2.STT+c2.ChatLLM+c2.Decisions+c2.TTS)) > 1e-9 {
		t.Errorf("expected total sum, got %f", c2.Total)
	}
	_ = expectedSTT
}

func floatPtr(f float64) *float64 { return &f }
