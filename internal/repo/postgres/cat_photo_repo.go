package postgres

import (
    "context"
    "errors"
    "time"

    "catforge/internal/app"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

type CatPhotoRepo struct { pool *pgxpool.Pool }
func NewCatPhotoRepo(pool *pgxpool.Pool) *CatPhotoRepo { return &CatPhotoRepo{pool:pool} }

func (r *CatPhotoRepo) Submit(ctx context.Context, userID, catID int64, fileID string) (app.CatPhotoRequest,error) {
    var result app.CatPhotoRequest
    tx,err:=r.pool.Begin(ctx);if err!=nil{return result,err};defer tx.Rollback(ctx)
    // Lock the user first to serialize new requests and moderation.
    var exists bool
    if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM cats WHERE id=$1 AND user_id=$2)`,catID,userID).Scan(&exists);err!=nil{return result,err}
    if !exists{return result,app.ErrCatPhotoNotFound}
    if _,err=tx.Exec(ctx,`INSERT INTO cat_photo_state(user_id) VALUES($1) ON CONFLICT DO NOTHING`,userID);err!=nil{return result,err}
    var freeUsed bool
    if err=tx.QueryRow(ctx,`SELECT free_used FROM cat_photo_state WHERE user_id=$1 FOR UPDATE`,userID).Scan(&freeUsed);err!=nil{return result,err}
    _=freeUsed
    err=tx.QueryRow(ctx,`INSERT INTO cat_photo_requests(user_id,cat_id,file_id,status) SELECT $1,$2,$3,'pending'
      WHERE NOT EXISTS(SELECT 1 FROM cat_photo_requests WHERE user_id=$1 AND status IN ('pending','awaiting_payment'))
      RETURNING id,user_id,cat_id,file_id,status`,userID,catID,fileID).Scan(&result.ID,&result.UserID,&result.CatID,&result.FileID,&result.Status)
    if errors.Is(err,pgx.ErrNoRows){return result,app.ErrCatPhotoPending}
    if err!=nil{return result,err}
    return result,tx.Commit(ctx)
}
func (r *CatPhotoRepo) Review(ctx context.Context, requestID, reviewerID int64, approve bool) (app.CatPhotoReviewResult,error) {
    var result app.CatPhotoReviewResult
    tx,err:=r.pool.Begin(ctx);if err!=nil{return result,err};defer tx.Rollback(ctx)
    var userID int64
    if err=tx.QueryRow(ctx,`SELECT user_id FROM cat_photo_requests WHERE id=$1`,requestID).Scan(&userID);err!=nil{
        if errors.Is(err,pgx.ErrNoRows){return result,app.ErrCatPhotoNotFound};return result,err
    }
    var freeUsed bool
    if err=tx.QueryRow(ctx,`SELECT free_used FROM cat_photo_state WHERE user_id=$1 FOR UPDATE`,userID).Scan(&freeUsed);err!=nil{return result,err}
    var req app.CatPhotoRequest
    if err=tx.QueryRow(ctx,`SELECT id,user_id,cat_id,file_id,status FROM cat_photo_requests WHERE id=$1 FOR UPDATE`,requestID).Scan(&req.ID,&req.UserID,&req.CatID,&req.FileID,&req.Status);err!=nil{return result,err}
    result.Request=req
    if req.Status!=app.CatPhotoPending{return result,app.ErrCatPhotoConflict}
    var catPresent bool
    if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM cats WHERE id=$1 AND user_id=$2)`,req.CatID,req.UserID).Scan(&catPresent);err!=nil{return result,err}
    status:=app.CatPhotoRejected
    if !catPresent {status=app.CatPhotoSuperseded
    }else if approve && freeUsed {status=app.CatPhotoAwaitingPayment
    }else if approve {status=app.CatPhotoApplied}
    if _,err=tx.Exec(ctx,`UPDATE cat_photo_requests SET status=$2,reviewed_at=now(),reviewer_telegram_id=$3 WHERE id=$1`,requestID,status,reviewerID);err!=nil{return result,err}
    result.Request.Status=status
    result.NeedsPayment=status==app.CatPhotoAwaitingPayment
    result.Applied=status==app.CatPhotoApplied
    if result.Applied {
        if _,err=tx.Exec(ctx,`UPDATE cat_photo_state SET free_used=true,current_cat_id=$2,current_file_id=$3,current_request_id=$4 WHERE user_id=$1`,req.UserID,req.CatID,req.FileID,req.ID);err!=nil{return result,err}
    }
    return result,tx.Commit(ctx)
}
func (r *CatPhotoRepo) Current(ctx context.Context,userID,catID int64)(string,error) {
    var fileID string
    err:=r.pool.QueryRow(ctx,`SELECT current_file_id FROM cat_photo_state WHERE user_id=$1 AND current_cat_id=$2`,userID,catID).Scan(&fileID)
    if errors.Is(err,pgx.ErrNoRows){return "",nil}
    return fileID,err
}
func (r *CatPhotoRepo) AwaitingReview(ctx context.Context,limit int)([]app.CatPhotoRequest,error){
    if limit<=0||limit>100{limit=20}
    rows,err:=r.pool.Query(ctx,`SELECT id,user_id,cat_id,file_id,status FROM cat_photo_requests WHERE status='pending' ORDER BY id LIMIT $1`,limit)
    if err!=nil{return nil,err};defer rows.Close()
    var out []app.CatPhotoRequest
    for rows.Next(){var p app.CatPhotoRequest;if err=rows.Scan(&p.ID,&p.UserID,&p.CatID,&p.FileID,&p.Status);err!=nil{return nil,err};out=append(out,p)}
    return out,rows.Err()
}
func (r *CatPhotoRepo) CreateInvoice(ctx context.Context,requestID,telegramID int64,payload string,now time.Time)(app.CatPhotoInvoice,error){
    var inv app.CatPhotoInvoice
    tx,err:=r.pool.Begin(ctx);if err!=nil{return inv,err};defer tx.Rollback(ctx)
    var userID int64
    var status app.CatPhotoStatus
    err=tx.QueryRow(ctx,`SELECT user_id,status FROM cat_photo_requests WHERE id=$1 FOR UPDATE`,requestID).Scan(&userID,&status)
    if err!=nil{return inv,err}
    var actualTG int64
    if err=tx.QueryRow(ctx,`SELECT telegram_id FROM users WHERE id=$1`,userID).Scan(&actualTG);err!=nil{return inv,err}
    if status!=app.CatPhotoAwaitingPayment||telegramID!=actualTG{return inv,app.ErrCatPhotoConflict}
    err=tx.QueryRow(ctx,`INSERT INTO cat_photo_invoices(request_id,telegram_id,invoice_payload,stars,expires_at)
        VALUES($1,$2,$3,10,$4) ON CONFLICT(request_id) DO UPDATE SET request_id=EXCLUDED.request_id
        RETURNING request_id,telegram_id,invoice_payload,stars,expires_at`,requestID,telegramID,payload,now.Add(30*24*time.Hour)).
        Scan(&inv.RequestID,&inv.TelegramID,&inv.Payload,&inv.Stars,&inv.ExpiresAt)
    if err!=nil{return inv,err}
    return inv,tx.Commit(ctx)
}
func (r *CatPhotoRepo) Checkout(ctx context.Context,queryID string,p app.CatPhotoPayment,now time.Time)(bool,error){
    if queryID==""||p.TelegramID<=0||p.Currency!="XTR"||p.Stars!=app.CatPhotoChangeStars{return false,nil}
    tag,err:=r.pool.Exec(ctx,`UPDATE cat_photo_invoices AS i SET pre_checkout_query_id=$1
        FROM cat_photo_requests AS req
        WHERE req.id=i.request_id AND req.status='awaiting_payment'
          AND i.invoice_payload=$2 AND i.telegram_id=$3 AND i.stars=$4 AND i.expires_at>$5
          AND (i.pre_checkout_query_id IS NULL OR i.pre_checkout_query_id=$1)
          AND NOT EXISTS(SELECT 1 FROM cat_photo_payments p WHERE p.invoice_payload=i.invoice_payload)`,queryID,p.Payload,p.TelegramID,p.Stars,now)
    return tag.RowsAffected()==1,err
}
func (r *CatPhotoRepo) RecordPayment(ctx context.Context,p app.CatPhotoPayment,now time.Time)error{
    if p.TelegramID<=0||p.ChargeID==""{return app.ErrCatPhotoInvalid}
    tx,err:=r.pool.Begin(ctx);if err!=nil{return err};defer tx.Rollback(ctx)
    var reqID int64;var tg int64;var stars int;var expires time.Time
    reason:=""
    err=tx.QueryRow(ctx,`SELECT request_id,telegram_id,stars,expires_at FROM cat_photo_invoices WHERE invoice_payload=$1 FOR UPDATE`,p.Payload).Scan(&reqID,&tg,&stars,&expires)
    if errors.Is(err,pgx.ErrNoRows){reason="unknown_invoice"}else if err!=nil{return err}
    if reason==""&&(tg!=p.TelegramID||stars!=p.Stars||p.Currency!="XTR"){reason="invoice_mismatch"}
    var other bool
    if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM cat_photo_payments WHERE invoice_payload=$1 AND telegram_payment_charge_id<>$2)`,p.Payload,p.ChargeID).Scan(&other);err!=nil{return err}
    if other{reason="multiple_charges"}
    var storedReq *int64
    if reason==""{storedReq=&reqID}
    state:="paid"
    if p.Refunded{state="refunded"}
    _,err=tx.Exec(ctx,`INSERT INTO cat_photo_payments(request_id,telegram_id,invoice_payload,currency,stars,telegram_payment_charge_id,status,review_reason,refunded_at)
         VALUES($1,$2,$3,$4,$5,$6,$7,$8,CASE WHEN $7='refunded' THEN $9::timestamptz ELSE NULL END)
         ON CONFLICT(telegram_payment_charge_id) DO NOTHING`,storedReq,p.TelegramID,p.Payload,p.Currency,p.Stars,p.ChargeID,state,reason,now)
    if err!=nil{return err}
    var id int64;var previous string;var storedTG int64;var storedStars int;var storedPayload,storedCurrency,storedStatus string
    err=tx.QueryRow(ctx,`SELECT id,review_reason,telegram_id,stars,invoice_payload,currency,status FROM cat_photo_payments WHERE telegram_payment_charge_id=$1 FOR UPDATE`,p.ChargeID).
         Scan(&id,&previous,&storedTG,&storedStars,&storedPayload,&storedCurrency,&storedStatus)
    if err!=nil{return err}
    if storedTG!=p.TelegramID||storedStars!=p.Stars||storedPayload!=p.Payload||storedCurrency!=p.Currency{reason="conflicting_receipt"}
    if reason!=""||previous!="" {
        if reason!=""{_,err=tx.Exec(ctx,`UPDATE cat_photo_payments SET review_reason=$2 WHERE id=$1`,id,reason);if err!=nil{return err}}
        if err=tx.Commit(ctx);err!=nil{return err}
        return app.ErrPaymentMismatch
    }
    if p.Refunded{
        if _,err=tx.Exec(ctx,`UPDATE cat_photo_payments SET status='refunded',refunded_at=COALESCE(refunded_at,$2) WHERE id=$1`,id,now);err!=nil{return err}
        if _,err=tx.Exec(ctx,`UPDATE cat_photo_state SET current_cat_id=NULL,current_file_id=NULL,current_request_id=NULL WHERE current_request_id=$1`,reqID);err!=nil{return err}
        if _,err=tx.Exec(ctx,`UPDATE cat_photo_requests SET status='refunded' WHERE id=$1 AND status IN ('awaiting_payment','applied')`,reqID);err!=nil{return err}
    }else if storedStatus!="refunded"{
        // Serialize with review and competing requests on the same user.
        var userID,catID int64;var fileID string;var status app.CatPhotoStatus
        if err=tx.QueryRow(ctx,`SELECT user_id,cat_id,file_id,status FROM cat_photo_requests WHERE id=$1 FOR UPDATE`,reqID).Scan(&userID,&catID,&fileID,&status);err!=nil{return err}
        var held bool
        if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM cats WHERE id=$1 AND user_id=$2)`,catID,userID).Scan(&held);err!=nil{return err}
        if status==app.CatPhotoAwaitingPayment&&held{
            if _,err=tx.Exec(ctx,`UPDATE cat_photo_state SET current_cat_id=$2,current_file_id=$3,current_request_id=$4 WHERE user_id=$1`,userID,catID,fileID,reqID);err!=nil{return err}
            if _,err=tx.Exec(ctx,`UPDATE cat_photo_requests SET status='applied' WHERE id=$1`,reqID);err!=nil{return err}
        } else if status==app.CatPhotoAwaitingPayment {
            if _,err=tx.Exec(ctx,`UPDATE cat_photo_requests SET status='superseded' WHERE id=$1`,reqID);err!=nil{return err}
            if _,err=tx.Exec(ctx,`UPDATE cat_photo_payments SET review_reason='paid_request_superseded' WHERE id=$1`,id);err!=nil{return err}
        }
    }
    return tx.Commit(ctx)
}

func (r *CatPhotoRepo) OpenForUser(ctx context.Context,userID int64)(*app.CatPhotoRequest,error) {
    var p app.CatPhotoRequest
    err:=r.pool.QueryRow(ctx,`SELECT id,user_id,cat_id,file_id,status FROM cat_photo_requests
        WHERE user_id=$1 AND status IN ('pending','awaiting_payment') ORDER BY id DESC LIMIT 1`,userID).
        Scan(&p.ID,&p.UserID,&p.CatID,&p.FileID,&p.Status)
    if errors.Is(err,pgx.ErrNoRows){return nil,nil}
    if err!=nil{return nil,err}
    return &p,nil
}
func (r *CatPhotoRepo) TelegramUserID(ctx context.Context,userID int64)(int64,error) {
    var id int64
    err:=r.pool.QueryRow(ctx,`SELECT telegram_id FROM users WHERE id=$1`,userID).Scan(&id)
    return id,err
}
