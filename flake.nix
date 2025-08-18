{
  description = "Shardul's macOS Configuration";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    nix-darwin.url = "github:LnL7/nix-darwin";
    nix-darwin.inputs.nixpkgs.follows = "nixpkgs";
    home-manager.url = "github:nix-community/home-manager";
    home-manager.inputs.nixpkgs.follows = "nixpkgs";
    nix-homebrew.url = "github:zhaofengli-wip/nix-homebrew";
    jjui.url = "github:idursun/jjui";
    jjui.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs =
    {
      self,
      nix-darwin,
      home-manager,
      nix-homebrew,
      jjui,
      nixpkgs,
      ...
    }:
    let
      # Build the secrets package
      secretsPkg = pkgs: pkgs.callPackage ./secrets { };

      mkDarwinConfiguration =
        hostname:
        { pkgs, ... }:
        {
          nixpkgs.config.allowUnfree = true;

          # List packages installed in system profile
          environment.systemPackages = with pkgs; [
            # CLI tools
            bat
            fd
            fzf
            gh
            git
            delta
            jq
            neovim
            nodejs
            bun
            ripgrep
            stow
            universal-ctags
            zoxide
            btop
            direnv
            mise
            age
            python311Packages.uv
            uv
            jujutsu
            jjui.packages.${pkgs.system}.default
            difftastic
            lowdown
            darwin.trash
            hyperfine
            zig
            wget
            switchaudio-osx
            blueutil
            _1password-cli
            google-cloud-sdk
            bottom
            nixfmt-classic
            tmux
            pam-reattach
            reattach-to-user-namespace
            mosh
            atuin
            yazi
            (secretsPkg pkgs)
            nil
            nixd
            sourcekit-lsp
            ollama
            chezmoi

            # macos gui apps
            aerospace
            whatsapp-for-mac
            istat-menus
            bartender
            vscode
          ];

          # Homebrew packages that aren't available in nixpkgs
          homebrew = {
            enable = true;
            brews = [
              "libyaml"
              "openssl"
            ];
            casks = [
              "google-chrome"
              "1password"
              "hammerspoon"
              "raycast"
              "spotify"
              "zoom"
              "anki"
              "cursor"
              "homerow"
              "cleanshot"
              "dropbox"
              "google-drive"
              "vlc"
              "zwift"
              "dash"
              "slack"
              "zed"
              "ghostty"
              "discord"
              "obsidian"
              "orbstack"
              "claude"
              "activitywatch"
            ];
            masApps = {
              "Infuse" = 1136220934;
            };
            onActivation.cleanup = "zap";
            onActivation.autoUpdate = false;
            onActivation.upgrade = false;
          };

          # Set the hostname based on configuration
          networking.computerName = hostname;
          networking.hostName = hostname;

          # macOS system configuration
          system.defaults = {
            # Dock settings
            dock.autohide = true; # Automatically hide and show the Dock
            dock.mru-spaces = false; # Don't automatically rearrange spaces based on most recent use
            dock.minimize-to-application = true; # Minimize windows into application icon instead of Dock
            dock.mineffect = "scale"; # Minimization effect (options: "genie", "scale")
            dock.launchanim = false; # Disable launch animations when opening applications
            dock.expose-animation-duration = 0.0; # Disable Mission Control animation (0.0 = instant)
            dock.orientation = "bottom"; # Dock position (options: "bottom", "left", "right")
            dock.show-recents = false; # Don't show recent applications in Dock
            dock.static-only = true; # Show only running applications in the Dock

            # Finder settings
            finder.CreateDesktop = false; # Don't show icons on the desktop
            finder.FXPreferredViewStyle = "Nlsv"; # Default view style ("Nlsv"=List, "icnv"=Icon, "clmv"=Column, "Flwv"=CoverFlow)
            finder.ShowPathbar = true; # Show path bar at bottom of Finder windows

            # Global system preferences
            NSGlobalDomain.InitialKeyRepeat = 15; # Delay until key repeat starts (lower = faster)
            NSGlobalDomain.KeyRepeat = 1; # Speed of key repeat (lower = faster)
            NSGlobalDomain.ApplePressAndHoldEnabled = false; # Disable press-and-hold for keys (enables key repeat)
            NSGlobalDomain.AppleKeyboardUIMode = 3; # Full keyboard access (3 = all controls)
            NSGlobalDomain."com.apple.keyboard.fnState" = false; # Use F1-F12 as standard function keys
            NSGlobalDomain."com.apple.mouse.tapBehavior" = 1; # Trackpad tap to click
            NSGlobalDomain."com.apple.trackpad.trackpadCornerClickBehavior" = 1; # Trackpad corner click behavior
            NSGlobalDomain.NSAutomaticCapitalizationEnabled = false; # Disable automatic capitalization
            NSGlobalDomain.NSAutomaticSpellingCorrectionEnabled = false; # Disable automatic spelling correction
            NSGlobalDomain.NSAutomaticQuoteSubstitutionEnabled = false; # Disable smart quotes
            NSGlobalDomain.NSAutomaticDashSubstitutionEnabled = false; # Disable smart dashes
            NSGlobalDomain.NSAutomaticPeriodSubstitutionEnabled = false; # Disable automatic period substitution
            NSGlobalDomain.NSAutomaticInlinePredictionEnabled = false; # Disable inline predictive text
            NSGlobalDomain.NSDocumentSaveNewDocumentsToCloud = false; # Save to disk by default, not iCloud
            NSGlobalDomain.NSNavPanelExpandedStateForSaveMode = true; # Expand save panel by default
            NSGlobalDomain.AppleInterfaceStyle = "Dark"; # Dark mode

            # Menu bar settings
            menuExtraClock.Show24Hour = true; # Use 24-hour time format
          };

          # Custom user preferences
          # Used for macOS preferences that don't have dedicated nix-darwin options
          system.defaults.CustomUserPreferences = {
            "com.apple.symbolichotkeys" = {
              AppleSymbolicHotKeys = {
                # Disable 'Cmd + Space' for Spotlight Search (to use with Raycast)
                "64" = {
                  enabled = false;
                };
                # Disable 'Cmd + Alt + Space' for Finder search window
                "65" = {
                  enabled = false;
                };
              };
            };
          };

          # Primary user configuration
          system.primaryUser = "shardul";

          # Fix build user group ID
          # ids.gids.nixbld = 350;

          # Auto upgrade nix package and the daemon service
          nix.package = pkgs.nix;

          # managed by determinate
          nix.enable = false;

          # Necessary for using flakes on this system
          nix.extraOptions = ''
            experimental-features = nix-command flakes
            trusted-users = root @admin shardul
          '';
          # nix.settings.trusted-users = [ "root" "@admin" "shardul" ];

          # Enable fish shell
          programs.fish = {
            enable = true;
            useBabelfish = true;
          };

          # Enable Touch ID for sudo authentication
          security.pam.services.sudo_local = {
            touchIdAuth = true;
            reattach = true; # Enable pam_reattach for Touch ID support in tmux
          };

          networking.knownNetworkServices = [
            "AX88179A"
            "Thunderbolt Bridge"
            "Wi-Fi"
            "Tailscale"
          ];

          services.tailscale = {
            enable = true;
            overrideLocalDns = true;
          };

          # Post-activation script to create symlinks for compatibility
          system.activationScripts.postActivation.text = ''
            # Create symlinks to make Nix binaries available in standard locations
            # This helps with scripts and tools that expect binaries in /usr/local

            # Remove the incorrectly created symlinks if they exist
            [ -L /usr/local/bin/bin ] && rm /usr/local/bin/bin
            [ -L /usr/local/lib/lib ] && rm /usr/local/lib/lib

            # Create individual symlinks for each binary in Nix store
            for f in /run/current-system/sw/bin/*; do
              if [ -e "$f" ]; then
                ln -sfn "$f" /usr/local/bin/
              fi
            done

            for f in /run/current-system/sw/lib/*; do
              if [ -e "$f" ]; then
                ln -sfn "$f" /usr/local/lib/
              fi
            done
          '';

          # Set Git commit hash for darwin-version
          system.configurationRevision = self.rev or self.dirtyRev or null;

          # Used for backwards compatibility
          system.stateVersion = 5;

          # The platform the configuration will be used on
          nixpkgs.hostPlatform = "aarch64-darwin";

          # Users configuration
          users.users.shardul = {
            name = "shardul";
            home = "/Users/shardul";
            shell = pkgs.fish;
          };
          services.karabiner-elements = {
            enable = true;
            package = pkgs.karabiner-elements.overrideAttrs (old: {
              version = "14.13.0";

              src = pkgs.fetchurl {
                inherit (old.src) url;
                hash = "sha256-gmJwoht/Tfm5qMecmq1N6PSAIfWOqsvuHU8VDJY8bLw=";
              };

              dontFixup = true;
            });
          };

          # Home Manager configuration
          home-manager.backupFileExtension = "backup";
          home-manager.useGlobalPkgs = true;
          home-manager.useUserPackages = true;
          home-manager.users.shardul =
            { ... }:
            {
              home.stateVersion = "23.05";
            };
        };
    in
    {
      darwinConfigurations."turbochardo" = nix-darwin.lib.darwinSystem {
        modules = [
          (mkDarwinConfiguration "turbochardo")
          nix-homebrew.darwinModules.nix-homebrew
          {
            nix-homebrew = {
              enable = true;
              enableRosetta = true;
              user = "shardul";
              autoMigrate = true;
            };
          }
          home-manager.darwinModules.home-manager
        ];
      };

      darwinConfigurations."baricbook" = nix-darwin.lib.darwinSystem {
        modules = [
          (mkDarwinConfiguration "baricbook")
          nix-homebrew.darwinModules.nix-homebrew
          {
            nix-homebrew = {
              enable = true;
              enableRosetta = true;
              user = "shardul";
              autoMigrate = true;
            };
          }
          home-manager.darwinModules.home-manager
        ];
      };

      # NixOS configuration for LXC container
      nixosConfigurations.proxmox-lxc = nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        modules = [
          home-manager.nixosModules.home-manager
          (
            { modulesPath, pkgs, ... }:
            {
              imports = [ (modulesPath + "/virtualisation/proxmox-lxc.nix") ];

              # Basic system configuration
              system.stateVersion = "24.11";

              # Proxmox LXC specific settings
              proxmoxLXC = {
                manageNetwork = false;
                privileged = false; # Set to true if needed
              };

              # Disable sandboxing for LXC
              nix.settings.sandbox = false;

              # Disable documentation to avoid man-cache build issues in container
              documentation.enable = false;
              documentation.nixos.enable = false;
              documentation.man.enable = false;
              documentation.info.enable = false;
              documentation.doc.enable = false;

              # Networking
              networking.hostName = "nixos";

              # Enable flakes
              nix.settings.experimental-features = [
                "nix-command"
                "flakes"
              ];

              # Allow unfree packages
              nixpkgs.config.allowUnfree = true;

              # System packages (CLI tools only)
              environment.systemPackages = with pkgs; [
                # Core CLI tools
                bat
                fd
                fzf
                gh
                git
                delta
                jq
                neovim
                nodejs
                bun
                ripgrep
                stow
                universal-ctags
                zoxide
                btop
                direnv
                mise
                age
                python311Packages.uv
                uv
                jujutsu
                jjui.packages.${pkgs.system}.default
                difftastic
                lowdown
                hyperfine
                zig
                wget
                google-cloud-sdk
                bottom
                nixfmt-classic
                tmux
                mosh
                atuin
                yazi
                nil
                nixd

                # Development tools
                gcc
                gnumake
                openssh
                curl
                tree
                htop
                ncdu
              ];

              # Enable SSH with initial access (tighten after setup)
              services.openssh = {
                enable = true;
                openFirewall = true;
                settings = {
                  PermitRootLogin = "yes"; # Change to "no" after initial setup
                  PasswordAuthentication = true; # Change to false after adding SSH keys
                  PermitEmptyPasswords = "yes"; # Remove after initial setup
                };
              };

              # Allow null password for initial SSH access
              security.pam.services.sshd.allowNullPassword = true;

              # Enable tailscale
              services.tailscale.enable = true;

              # User configuration
              users.users.shardul = {
                isNormalUser = true;
                extraGroups = [ "wheel" ];
                shell = pkgs.fish;
              };

              # Enable fish shell
              programs.fish.enable = true;

              # Home Manager configuration
              home-manager = {
                useGlobalPkgs = true;
                useUserPackages = true;
                users.shardul =
                  { ... }:
                  {
                    home.stateVersion = "24.11";

                    # Git configuration
                    programs.git.enable = true;
                    home.file = {
                      ".hushlogin" = {
                        text = "";
                      };

                      # Create Documents directory
                      "Documents/.keep" = {
                        text = "";
                      };
                    };

                    # Programs configuration
                    programs.fzf = {
                      enable = true;
                      enableFishIntegration = true;
                    };

                    programs.zoxide = {
                      enable = true;
                      enableFishIntegration = true;
                    };

                    programs.direnv = {
                      enable = true;
                      enableFishIntegration = true;
                      nix-direnv.enable = true;
                    };

                    programs.atuin = {
                      enable = true;
                      enableFishIntegration = true;
                    };

                    programs.bat = {
                      enable = true;
                      config = {
                        theme = "gruvbox-dark";
                      };
                    };
                  };
              };
            }
          )
        ];
      };
    };
}
