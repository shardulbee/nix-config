{ lib, buildGoModule }:

buildGoModule rec {
  pname = "secrets";
  version = "1.0.0";

  src = ./.;

  vendorHash = "sha256-H0jabIQ5EgXvpOr0FOMUXSziXuzRXYzMEkhM2EagVKU=";

  meta = with lib; {
    description = "Age-based secrets management tool";
    homepage = "https://github.com/shardul/dotfiles";
    license = licenses.mit;
    maintainers = [ ];
    mainProgram = "secrets";
  };
}