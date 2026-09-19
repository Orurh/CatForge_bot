package app
import("context";"encoding/json";"fmt";"os";"testing";"time";"catforge/internal/ai";"catforge/internal/domain")
func TestTemporaryComedyPreview(t *testing.T){
 if os.Getenv("CATFORGE_LIVE_AI_TEST") != "1" { t.Skip("set CATFORGE_LIVE_AI_TEST=1 to spend real AI quota") }
 provider,err:=ai.NewCodexProvider("/home/orurh/Документы/VSCode/catforge-bot/.codex-bridge/bridge.sock","gpt-5.6-luna");if err!=nil{t.Fatal(err)}
 voice:=ai.NewGateway(provider,nil,nil,30*time.Second)
 wanted:=map[string]bool{"наезд с обратным эффектом":true,"пошлая двусмысленность":true,"нежность по-дворовому":true,"помощь от главного источника проблем":true}
 results:=[]map[string]any{}
 for slot:=int64(0);len(wanted)>0 && slot<100;slot++{
  topic,scene:=idleBanterScene(slot,true);if !wanted[topic]{continue};delete(wanted,topic)
  request:=yardBanterRequest(&domain.Yard{HumorMode:domain.HumorBold},banterSpeaker{Name:"Игорь лох",Trait:domain.TraitCunning,HumorMode:domain.HumorBold},banterSpeaker{Name:"Джулиан",Trait:domain.TraitCunning,HumorMode:domain.HumorBold},&ai.YardBanterContext{Trigger:"idle",Topic:topic,Scene:scene,Variation:uint64(slot)},nil)
  request.EventFacts=[]string{"conversation_topic="+topic};start:=time.Now();r,err:=voice.GenerateYardBanter(context.Background(),request)
  row:=map[string]any{"case":topic,"text":r.Text,"fallback":r.Fallback,"seconds":time.Since(start).Seconds()};if err!=nil{row["error"]=err.Error()};results=append(results,row);out,_:=json.Marshal(row);fmt.Println(string(out))
 }
 out,_:=json.MarshalIndent(results,"","  ");os.WriteFile("/tmp/catforge-comedy-production-scenes.json",out,0600)
}
