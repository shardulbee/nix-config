OS := $(shell uname -s)
HOST := $(shell hostname -s)
CMD := $(if $(filter Darwin,$(OS)),sudo darwin-rebuild,sudo nixos-rebuild)

switch:
	$(CMD) switch --flake .#$(HOST)

.PHONY: switch
