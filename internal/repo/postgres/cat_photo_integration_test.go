package postgres

import (
    "context"
    "errors"
    "testing"
    "time"

    "catforge/internal/app"
    "catforge/internal/domain"
)

func TestCatPhotoModerationAndStarsIntegration(t *testing.T) {
    pool:=paymentTestPool(t) // isolated schema, migrations, no live Telegram
    ctx:=context.Background()
    uid,err:=NewUserRepo(pool).EnsureUser(ctx,987601)
    if err!=nil{t.Fatal(err)}
    cats:=NewCatRepo(pool)
    cat,err:=cats.Create(ctx,uid,"Тест",domain.BreedBritish,domain.TraitLazy,60,40,30,25)
    if err!=nil{t.Fatal(err)}
    repo:=NewCatPhotoRepo(pool)
    photos:=app.NewCatPhotoService(repo,app.SystemClock{})
    pending,err:=photos.Submit(ctx,uid,cat.ID,"telegram-photo-1")
    if err!=nil{t.Fatal(err)}
    if _,err=photos.Submit(ctx,uid,cat.ID,"duplicate");!errors.Is(err,app.ErrCatPhotoPending){t.Fatalf("duplicate request: %v",err)}
    if url,err:=photos.Current(ctx,uid,cat.ID);err!=nil||url!=""{t.Fatalf("unapproved photo visible: %s %v",url,err)}
    rejected,err:=photos.Review(ctx,pending.ID,998,true==false)
    if err!=nil||rejected.Applied||rejected.Request.Status!=app.CatPhotoRejected{t.Fatalf("reject: %+v %v",rejected,err)}
    approved,err:=photos.Submit(ctx,uid,cat.ID,"telegram-photo-free")
    if err!=nil{t.Fatal(err)}
    review,err:=photos.Review(ctx,approved.ID,998,true)
    if err!=nil||!review.Applied||review.NeedsPayment{t.Fatalf("free approval: %+v %v",review,err)}
    if _,err=photos.Review(ctx,approved.ID,998,true);!errors.Is(err,app.ErrCatPhotoConflict){t.Fatalf("duplicate approval: %v",err)}
    if current,err:=photos.Current(ctx,uid,cat.ID);err!=nil||current!="telegram-photo-free"{t.Fatalf("current free photo %q: %v",current,err)}
    next,err:=photos.Submit(ctx,uid,cat.ID,"telegram-photo-paid")
    if err!=nil{t.Fatal(err)}
    review,err=photos.Review(ctx,next.ID,998,true)
    if err!=nil||review.Applied||!review.NeedsPayment{t.Fatalf("paid approval: %+v %v",review,err)}
    if current,_:=photos.Current(ctx,uid,cat.ID);current!="telegram-photo-free"{t.Fatalf("changed before payment: %q",current)}
    invoice,err:=photos.CreateInvoice(ctx,next.ID,987601)
    if err!=nil||invoice.Stars!=10{t.Fatalf("invoice: %+v %v",invoice,err)}
    retry,err:=photos.CreateInvoice(ctx,next.ID,987601)
    if err!=nil||retry.Payload!=invoice.Payload{t.Fatalf("invoice not idempotent: %+v %v",retry,err)}
    pay:=app.CatPhotoPayment{TelegramID:987601,Currency:"XTR",Stars:10,Payload:invoice.Payload,ChargeID:"photo-charge-1"}
    for _,bad:=range []app.CatPhotoPayment{
        {TelegramID:987601,Currency:"XTR",Stars:11,Payload:invoice.Payload},
        {TelegramID:987602,Currency:"XTR",Stars:10,Payload:invoice.Payload},
        {TelegramID:987601,Currency:"USD",Stars:10,Payload:invoice.Payload},
    } {
        if ok,err:=photos.Checkout(ctx,"bad-checkout",bad);err!=nil||ok{t.Fatalf("bad checkout allowed: %v %v",ok,err)}
    }
    if ok,err:=photos.Checkout(ctx,"checkout-photo",pay);err!=nil||!ok{t.Fatalf("checkout: %v %v",ok,err)}
    if current,_:=photos.Current(ctx,uid,cat.ID);current!="telegram-photo-free"{t.Fatal("checkout changed photo without receipt")}
    if err=photos.RecordPayment(ctx,pay);err!=nil{t.Fatal(err)}
    if err=photos.RecordPayment(ctx,pay);err!=nil{t.Fatalf("duplicate receipt: %v",err)}
    if current,err:=photos.Current(ctx,uid,cat.ID);err!=nil||current!="telegram-photo-paid"{t.Fatalf("photo after payment: %q %v",current,err)}
    if open,err:=photos.OpenForUser(ctx,uid);err!=nil||open!=nil{t.Fatalf("paid request still open: %+v %v",open,err)}
    var count int
    if err=pool.QueryRow(ctx,`SELECT count(*) FROM cat_photo_payments WHERE invoice_payload=$1`,invoice.Payload).Scan(&count);err!=nil||count!=1{t.Fatalf("duplicate charge ledger: %d %v",count,err)}
    if err=photos.RecordPayment(ctx,app.CatPhotoPayment{TelegramID:987601,Currency:"XTR",Stars:10,Payload:"missing",ChargeID:"unknown-charge"});!errors.Is(err,app.ErrPaymentMismatch){t.Fatalf("unknown receipt: %v",err)}
    if err=cats.DeleteByUserID(ctx,uid);err!=nil{t.Fatal(err)}
    fresh,err:=cats.Create(ctx,uid,"Другой кот",domain.BreedBritish,domain.TraitLazy,60,40,30,25)
    if err!=nil{t.Fatal(err)}
    if current,err:=photos.Current(ctx,uid,fresh.ID);err!=nil||current!=""{t.Fatalf("old photo leaked to new cat: %q %v",current,err)}
    third,err:=photos.Submit(ctx,uid,fresh.ID,"third-photo")
    if err!=nil{t.Fatal(err)}
    decision,err:=photos.Review(ctx,third.ID,998,true)
    if err!=nil||!decision.NeedsPayment{t.Fatalf("free right incorrectly reset: %+v %v",decision,err)}
}

func TestCatPhotoInvoiceCannotBePurchasedByOtherTelegramUser(t *testing.T) {
    pool:=paymentTestPool(t)
    ctx:=context.Background()
    uid,err:=NewUserRepo(pool).EnsureUser(ctx,765401);if err!=nil{t.Fatal(err)}
    cat,err:=NewCatRepo(pool).Create(ctx,uid,"Кот",domain.BreedBritish,domain.TraitLazy,60,40,30,25);if err!=nil{t.Fatal(err)}
    photos:=app.NewCatPhotoService(NewCatPhotoRepo(pool),app.SystemClock{})
    first,err:=photos.Submit(ctx,uid,cat.ID,"a");if err!=nil{t.Fatal(err)}
    if _,err=photos.Review(ctx,first.ID,10,true);err!=nil{t.Fatal(err)}
    second,err:=photos.Submit(ctx,uid,cat.ID,"b");if err!=nil{t.Fatal(err)}
    if _,err=photos.Review(ctx,second.ID,10,true);err!=nil{t.Fatal(err)}
    if _,err=photos.CreateInvoice(ctx,second.ID,99999);!errors.Is(err,app.ErrCatPhotoConflict){t.Fatalf("wrong payer: %v",err)}
    if _,err=photos.CreateInvoice(ctx,second.ID,765401);err!=nil{t.Fatal(err)}
    _=time.Second
}
