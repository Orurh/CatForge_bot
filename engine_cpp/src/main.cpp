#include "catforge/grpc_service.hpp"

#include <grpcpp/grpcpp.h>

#include <iostream>
#include <memory>
#include <string>

int main(int argc, char *argv[]) {
  const std::string address = argc > 1 ? argv[1] : "0.0.0.0:50051";
  catforge::engine::GameEngineService service;

  grpc::ServerBuilder builder;
  builder.AddListeningPort(address, grpc::InsecureServerCredentials());
  builder.RegisterService(&service);
  auto server = builder.BuildAndStart();
  if (!server) {
    std::cerr << "failed to start CatForge game engine on " << address << '\n';
    return 1;
  }

  std::cout << "CatForge game engine listening on " << address << '\n';
  server->Wait();
  return 0;
}
