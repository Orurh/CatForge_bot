package telegram

import (
    "context"
    "errors"
    "fmt"
    "strconv"
    "strings"
    "time"

    "catforge/internal/app"
    "catforge/internal/domain"
    "catforge/internal/transport/telegram/views"
)

const catPhotoPendingKind = "await_cat_photo"

func (r *Router) ConfigureCatPhotos(enabled bool,reviewerID int64) {
    r.catPhotoEnabled = enabled && reviewerID>0
    r.catPhotoReviewerID = reviewerID
}
func (r *Router) catPhotoURL(ctx context.Context,cat *domain.Cat)string{
    original:=views.CatAvatarURL(r.publicBaseURL,cat)
    if !r.catPhotoEnabled ||r.app.CatPhotos==nil{return original}
    fileID,err:=r.app.CatPhotos.Current(ctx,cat.UserID,cat.ID)
    if err!=nil{r.log.Warn("cat photo lookup failed");return original}
    if fileID!=""{return fileID}
    return original
}
func (r *Router) requestCatPhoto(ctx context.Context,tgc *tgCtx){
    if tgc.chatType!="private" {r.openPrivatePersonalUI(ctx,tgc);return}
    if !r.catPhotoEnabled||r.app.CatPhotos==nil {r.sendText(ctx,tgc.chatID,"Смена фотографии временно недоступна.");return}
    cat,err:=r.app.Profile.GetCat(ctx,tgc.userID)
    if err!=nil||cat==nil{r.sendText(ctx,tgc.chatID,"Сначала создай кота через /start.");return}
    open,err:=r.app.CatPhotos.OpenForUser(ctx,tgc.userID)
    if err!=nil{r.sendText(ctx,tgc.chatID,"Не удалось проверить заявку. Попробуй позже.");return}
    if open!=nil {
        if open.Status==app.CatPhotoAwaitingPayment {r.sendCatPhotoInvoice(ctx,tgc.tgID,open.ID);return}
        r.sendText(ctx,tgc.chatID,"Твоя фотография уже ожидает проверки. Отменить отправленную заявку пока нельзя.")
        return
    }
    _,err=r.send.ForceReply(ctx,tgc.chatID,
        "📷 Пришли фото кота ответом на это сообщение.\n\nПервое одобренное изменение бесплатно. Следующие — по 10 ⭐ после модерации.\n\nНе присылай фотографии людей, чужие приватные данные или запрещённые изображения. Модератор увидит фото до публикации.\n\nОтмена: /skip.","Фото кота")
    if err!=nil{r.sendText(ctx,tgc.chatID,"Не удалось запросить фото.");return}
    if err=r.app.Users.SavePendingInput(ctx,app.PendingInput{UserID:tgc.userID,ChatID:tgc.chatID,Kind:catPhotoPendingKind,ExpiresAt:tgc.now.Add(15*time.Minute)});err!=nil{
        r.sendText(ctx,tgc.chatID,"Не удалось сохранить запрос. Попробуй /photo позже.")
    }
}
func (r *Router) tryConsumeCatPhoto(ctx context.Context,tgc *tgCtx,m *Message)bool{
    if !r.catPhotoEnabled||r.app.CatPhotos==nil||tgc.chatType!="private"||m==nil||len(m.Photo)==0{return false}
    pending,err:=r.app.Users.GetPendingInput(ctx,tgc.userID,tgc.chatID)
    if err!=nil||pending==nil||pending.Kind!=catPhotoPendingKind{return false}
    photo:=m.Photo[len(m.Photo)-1]
    if !app.ValidCatPhotoFileID(photo.FileID)||(photo.FileSize>10<<20)||(photo.Width>10000)||(photo.Height>10000){
        r.sendText(ctx,tgc.chatID,"Фото слишком большое или некорректное. Пришли другое изображение.")
        return true
    }
    cat,err:=r.app.Profile.GetCat(ctx,tgc.userID)
    if err!=nil||cat==nil{r.sendText(ctx,tgc.chatID,"Кот не найден.");return true}
    req,err:=r.app.CatPhotos.Submit(ctx,tgc.userID,cat.ID,photo.FileID)
    if err!=nil{
        if errors.Is(err,app.ErrCatPhotoPending){r.sendText(ctx,tgc.chatID,"Заявка уже ожидает рассмотрения.")} else {r.sendText(ctx,tgc.chatID,"Не удалось сохранить заявку. Попробуй позже.")}
        return true
    }
    _=r.app.Users.ClearPendingInput(ctx,tgc.userID,tgc.chatID)
    r.sendText(ctx,tgc.chatID,"📨 Фото отправлено на проверку. До одобрения останется прежний аватар.")
    r.notifyCatPhotoReviewer(ctx,req)
    return true
}
func catPhotoAdminKeyboard(requestID int64)map[string]any{
    n:=strconv.FormatInt(requestID,10)
    return map[string]any{"inline_keyboard":[][]map[string]any{
        {{"text":"✅ Одобрить","callback_data":"photo:admin:"+n+":yes"},
         {"text":"❌ Отклонить","callback_data":"photo:admin:"+n+":no"}},
    }}
}
func (r *Router) notifyCatPhotoReviewer(ctx context.Context,req app.CatPhotoRequest){
    if r.catPhotoReviewerID<=0{return}
    caption:=fmt.Sprintf("📷 Заявка #%d\nПользователь: %d\nКот: %d\nФото не опубликовано. Проверь содержимое.",req.ID,req.UserID,req.CatID)
    if err:=r.send.PhotoWithKeyboard(ctx,r.catPhotoReviewerID,req.FileID,caption,catPhotoAdminKeyboard(req.ID));err!=nil{
        r.log.Warn("photo review notification failed; use /photoqueue to recover")
    }
}
func (r *Router) photoQueue(ctx context.Context,tgc *tgCtx){
    if !r.catPhotoEnabled||tgc.chatType!="private"||tgc.tgID!=r.catPhotoReviewerID{return}
    pending,err:=r.app.CatPhotos.AwaitingReview(ctx)
    if err!=nil{r.sendText(ctx,tgc.chatID,"Не удалось получить очередь.");return}
    if len(pending)==0{r.sendText(ctx,tgc.chatID,"Нет заявок на рассмотрении.");return}
    for _,req:=range pending{r.notifyCatPhotoReviewer(ctx,req)}
}
func (r *Router) handleCatPhotoAdminCallback(ctx context.Context,cbc *cbCtx,callbackID string)bool{
    if !strings.HasPrefix(cbc.data,"photo:admin:"){return false}
    if !r.catPhotoEnabled||cbc.chatType!="private"||cbc.tgID!=r.catPhotoReviewerID {
        _=r.send.AnswerCallbackText(ctx,callbackID,"Нет доступа к модерации.")
        return true
    }
    parts:=strings.Split(cbc.data,":")
    if len(parts)!=4||(parts[3]!="yes"&&parts[3]!="no"){_ =r.send.AnswerCallbackText(ctx,callbackID,"Неверное действие.");return true}
    id,err:=strconv.ParseInt(parts[2],10,64)
    if err!=nil||id<=0 {_=r.send.AnswerCallbackText(ctx,callbackID,"Неверная заявка.");return true}
    result,err:=r.app.CatPhotos.Review(ctx,id,r.catPhotoReviewerID,parts[3]=="yes")
    if err!=nil{_ =r.send.AnswerCallbackText(ctx,callbackID,"Не удалось обработать или заявка уже закрыта.");return true}
    _=r.send.AnswerCallbackText(ctx,callbackID,"Решение сохранено.")
    userID,err:=r.app.CatPhotos.TelegramUserID(ctx,result.Request.UserID)
    if err!=nil{r.log.Warn("photo review owner lookup failed");return true}
    switch {
    case result.Applied:
        r.sendText(ctx,userID,"✅ Фото кота одобрено и установлено. Первая смена была бесплатной.")
    case result.NeedsPayment:
        r.sendText(ctx,userID,"✅ Фото одобрено. Для установки повторной фотографии потребуется 10 ⭐.")
        r.sendCatPhotoInvoice(ctx,userID,result.Request.ID)
    case result.Request.Status==app.CatPhotoSuperseded:
        r.sendText(ctx,userID,"Кот изменился до решения по фотографии. Заявка закрыта без оплаты.")
    default:
        r.sendText(ctx,userID,"❌ Фото отклонено. Аватар не изменился. Можешь прислать другое через /photo.")
    }
    return true
}
func (r *Router) sendCatPhotoInvoice(ctx context.Context,telegramID,requestID int64){
    invoice,err:=r.app.CatPhotos.CreateInvoice(ctx,requestID,telegramID)
    if err!=nil{r.sendText(ctx,telegramID,"Не удалось подготовить счёт. Повтори /photo позже.");return}
    err=r.send.Call(ctx,"sendInvoice",map[string]any{
        "chat_id":telegramID,
        "title":"Сменить фото кота",
        "description":"Одобренная модератором смена фотографии кота. Разовый платёж 10 ⭐. Возврат и поддержка: /paysupport.",
        "payload":invoice.Payload,
        "provider_token":"","currency":"XTR",
        "start_parameter":"cat_photo",
        "prices":[]map[string]any{{"label":"Смена фото кота","amount":invoice.Stars}},
    })
    if err!=nil{r.log.Warn("cat photo invoice send failed");r.sendText(ctx,telegramID,"Не удалось отправить счёт. Повтори /photo позже.")}
}
