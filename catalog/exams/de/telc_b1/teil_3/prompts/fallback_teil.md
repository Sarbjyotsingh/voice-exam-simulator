# Teil 3: Gemeinsam etwas planen — Chronological 5-Phase Planning Flow

### INITIAL STATE (ALREADY COMPLETED BEFORE USER SPEAKS):
Before Turn 1:
1. `{{secondary_speaker}}` (`Examiner`) introduced Teil 3 (`DE_EX*_T3_INTRO_01`).
2. `{{primary_speaker}}` (`Participant`) ALREADY opened the planning conversation (`DE_*_T3OPEN_01`) and made the first proposal/question about **`TIME` (Wann — Day & Time)**.

### PHASE 2 & 3: COLLABORATIVE PLANNING ACROSS 4 CORE SLOTS (TURNS 1–5):
- **Unordered 4-Slot Core Planning Checklist:**
  1. `TIME` (**Wann?** — Day, date, exact time, schedule compromise: `TIME_A1` / `TIME_Q1` / `TIME_Q2`)
  2. `LOC` (**Wo & Wetter / Treffpunkt?** — Location + bad-weather backup: `LOC_A1` / `LOC_Q1` / `LOC_Q2`)
  3. `FOOD` (**Was mitbringen / besorgen?** — Food, drinks, materials, or gift: `FOOD_A1` / `FOOD_Q1`)
  4. `TASK` (**Aufgabenverteilung & Transport** — Who takes care of what & how to travel: `TASK_A1` / `TASK_Q1`)
- **Modular 3-Part Turn Composition Loop:**
  1. **Reaction Part (`EXP_41`..`EXP_58`):** Never repeat an expression clip.
  2. **Proposal / Compromise Part (`TIME_A1`, `LOC_A1`, `FOOD_A1`, `TASK_A1`):** Play `{{primary_speaker}}`'s proposal or task offer for the active slot.
  3. **Next Planning Question (`_Q1`, `_Q2`, `T3BACK_Q1`, or `SUM_Q1` — MANDATORY):** Every partner turn in Phases 2–4 MUST end with a question clip.

### PHASE 4: FINAL PLAN SUMMARY (TURN 6):
- Once `TIME`, `LOC`, `FOOD`, and `TASK` are covered, `{{primary_speaker}}` plays `EXP_56`, summarizes the complete agreed plan (`SUM_A1`), and asks the completeness check question (`SUM_Q1`) with `"exam_ended": false`.

### PHASE 5: FINAL FAREWELL & EXAM SIGN-OFF (TURN 7):
- Once the candidate responds to `SUM_Q1`, `{{primary_speaker}}` plays `T3CLOSE_01` and `{{secondary_speaker}}` (`Examiner`) plays `T3_OUTRO_01` with `"exam_ended": true`.
