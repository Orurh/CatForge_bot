#pragma once

#include "gameengine/v1/game_engine.grpc.pb.h"

namespace catforge::engine {

class GameEngineService final
    : public gameengine::v1::GameEngineService::Service {
public:
  grpc::Status Progress(grpc::ServerContext *,
                        const gameengine::v1::ProgressRequest *,
                        gameengine::v1::ProgressResponse *) override;
  grpc::Status Train(grpc::ServerContext *context,
                     const gameengine::v1::TrainRequest *request,
                     gameengine::v1::TrainResponse *response) override;
  grpc::Status
  Expedition(grpc::ServerContext *context,
             const gameengine::v1::ExpeditionRequest *request,
             gameengine::v1::ExpeditionResponse *response) override;
  grpc::Status
  ResolveYardEvent(grpc::ServerContext *context,
                   const gameengine::v1::ResolveYardEventRequest *request,
                   gameengine::v1::ResolveYardEventResponse *response) override;
  grpc::Status Fight(grpc::ServerContext *context,
                     const gameengine::v1::FightRequest *request,
                     gameengine::v1::FightResponse *response) override;
};

} // namespace catforge::engine
