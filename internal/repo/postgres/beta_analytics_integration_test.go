package postgres

import (
	"context"
	"testing"
)

func TestBetaAnalyticsIntegration(t *testing.T) {
	pool := paymentTestPool(t)
	ctx := context.Background()
	// PostgreSQL timezone differs deliberately from the report's Moscow dates.
	_, err := pool.Exec(ctx, `
 INSERT INTO users(id,telegram_id) VALUES(1,101),(2,102),(3,103),(4,104);
 INSERT INTO cats(id,user_id,name,breed,trait,hp_base,atk_base,def_base,spd_base)
 SELECT id,id,'Cat'||id,'british','lazy',10,10,10,10 FROM users WHERE id <= 3;
 INSERT INTO yards(id,telegram_chat_id,name) VALUES(1,-101,'Active'),(2,-102,'Empty');
 INSERT INTO game_events(user_id,cat_id,yard_id,event_type,occurred_at)
 SELECT id,id,1,'cat_created',((now() AT TIME ZONE 'Europe/Moscow')::date - 20)::timestamp AT TIME ZONE 'Europe/Moscow'
 FROM cats;
 -- A later creation must not reset the original cohort.
 INSERT INTO game_events(user_id,event_type,occurred_at) VALUES(1,'cat_created',now()),(4,'cat_created',now());
 INSERT INTO game_events(user_id,cat_id,yard_id,event_type,occurred_at)
 SELECT c.id,c.id,1,'cat_trained',(((now() AT TIME ZONE 'Europe/Moscow')::date - 20 + d.n)::timestamp + interval '1 hour') AT TIME ZONE 'Europe/Moscow'
 FROM cats c CROSS JOIN (VALUES(0),(7)) d(n);
 INSERT INTO game_events(user_id,cat_id,yard_id,event_type,occurred_at)
 SELECT 1,1,1,'autonomous_cat_message',(((now() AT TIME ZONE 'Europe/Moscow')::date - 19)::timestamp + interval '1 hour') AT TIME ZONE 'Europe/Moscow';
 -- The author (user 2), not the speaker's owner (user 1), returned on D1.
 INSERT INTO game_events(user_id,cat_id,yard_id,event_type,occurred_at)
 SELECT 2,1,1,'human_reply_to_cat',(((now() AT TIME ZONE 'Europe/Moscow')::date - 19)::timestamp + interval '1 hour') AT TIME ZONE 'Europe/Moscow';
 INSERT INTO yard_events(id,yard_id,event_type,state,seed,starts_at,resolves_at,content_version)
 VALUES(1,1,'fish_truck','active',1,now()-interval '2 hours',now()+interval '1 hour',5),
       (2,1,'fish_truck','cancelled',2,now()-interval '4 hours',now()-interval '3 hours',5);
 INSERT INTO yard_event_choices(event_id,cat_id,choice_id) VALUES(1,1,'scout'),(2,1,'scout');
 UPDATE yard_event_choices SET choice_id='steal' WHERE event_id=1;
 INSERT INTO yard_fights(yard_id,cat_a_id,cat_b_id,cat_a_name,cat_b_name,winner_cat_id,loser_cat_id,winner_name,loser_name,seed,rounds,final_hp_a,final_hp_b,rules_version,content_version,created_at)
 VALUES(1,1,2,'Cat1','Cat2',1,2,'Cat1','Cat2',1,1,1,0,14,5,now()-interval '1 hour'),
       (1,1,2,'Cat1','Cat2',2,1,'Cat2','Cat1',2,1,0,1,14,5,now());
 `)
	if err != nil {
		t.Fatal(err)
	}
	var cohort, eligible, returned int
	err = pool.QueryRow(ctx, `SELECT cohort_users,eligible_users,returned_users FROM analytics_beta_user_cohorts WHERE cohort_date=(now() AT TIME ZONE 'Europe/Moscow')::date-20 AND day_number=1`).Scan(&cohort, &eligible, &returned)
	if err != nil || cohort != 3 || eligible != 3 || returned != 1 {
		t.Fatalf("D1: %d/%d/%d: %v", cohort, eligible, returned, err)
	}
	var immature bool
	err = pool.QueryRow(ctx, `SELECT eligible_users=0 AND retention_pct IS NULL FROM analytics_beta_user_cohorts WHERE cohort_date=(now() AT TIME ZONE 'Europe/Moscow')::date AND day_number=7`).Scan(&immature)
	if err != nil || !immature {
		t.Fatalf("immature cohort: %v %v", immature, err)
	}
	var activated, empty bool
	err = pool.QueryRow(ctx, `SELECT d7_eligible AND three_active_cats_d7 FROM analytics_beta_yard_activation WHERE yard_id=1`).Scan(&activated)
	if err != nil || !activated {
		t.Fatalf("yard activation: %v %v", activated, err)
	}
	err = pool.QueryRow(ctx, `SELECT activated_date IS NULL AND NOT d7_eligible AND three_active_cats_d7 IS NULL FROM analytics_beta_yard_activation WHERE yard_id=2`).Scan(&empty)
	if err != nil || !empty {
		t.Fatalf("empty yard hidden: %v %v", empty, err)
	}
	var repeats int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM analytics_beta_repeat_actions WHERE action='fight' AND second_at IS NOT NULL AND action_count=2`).Scan(&repeats)
	if err != nil || repeats != 2 {
		t.Fatalf("both fighters: %d %v", repeats, err)
	}
	var once bool
	err = pool.QueryRow(ctx, `SELECT action_count=1 AND second_at IS NULL FROM analytics_beta_repeat_actions WHERE user_id=1 AND action='event'`).Scan(&once)
	if err != nil || !once {
		t.Fatalf("choice dedupe: %v %v", once, err)
	}
	var trainings int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM analytics_beta_repeat_actions WHERE action='training' AND second_at IS NOT NULL`).Scan(&trainings)
	if err != nil || trainings != 3 {
		t.Fatalf("repeat training: %d %v", trainings, err)
	}
}
