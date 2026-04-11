class NightAgent < Formula
  desc "Runtime security layer for AI agents on macOS"
  homepage "https://github.com/pietroperona/night-agent"
  url "https://github.com/pietroperona/night-agent/archive/refs/tags/v0.2.2.tar.gz"
  sha256 "0019dfc4b32d63c1392aa264aed2253c1e0c2fb09216f8e2cc269bbfb8bb49b5"
  license "MIT"
  head "https://github.com/pietroperona/night-agent.git", branch: "main"

  depends_on "go" => :build
  depends_on :macos

  def install
    # Compila il binario principale Go
    system "go", "build", "-o", bin/"night-agent", "./cmd/guardian"

    # Compila lo shim C (intercettazione comandi via PATH)
    system "clang", "-o", libexec/"guardian-shim",
           "internal/shim/csrc/guardian_shim.c",
           "-Wall", "-Wextra", "-Wno-unused-parameter"

    # Compila la dylib DYLD_INSERT_LIBRARIES (opzionale, per intercettazione avanzata)
    system "clang", "-dynamiclib",
           "-o", lib/"guardian-intercept.dylib",
           "internal/intercept/csrc/guardian_intercept.c",
           "-Wall", "-Wextra", "-Wno-unused-parameter",
           "-current_version", "1.0",
           "-compatibility_version", "1.0"

    # Installa la policy di default
    pkgshare.install "configs"

    # Crea symlink per lo shim nella libexec
    (libexec/"shims").mkpath
  end

  def post_install
    # Copia la policy di default se non esiste già
    guardian_dir = Pathname.new(ENV["HOME"]) / ".night-agent"
    policy_dest = guardian_dir / "policy.yaml"
    policy_src = pkgshare / "configs" / "default_policy.yaml"

    unless policy_dest.exist?
      guardian_dir.mkpath
      FileUtils.cp policy_src, policy_dest
      policy_dest.chmod(0600)
    end
  end

  def caveats
    <<~EOS
      Night Agent è stato installato. Per completare la configurazione:

      1. Inizializza Night Agent:
           night-agent init

      2. Per le funzionalità sandbox, installa Docker Desktop:
           https://www.docker.com/products/docker-desktop/
         Avvialo almeno una volta manualmente dopo l'installazione.

      3. Riavvia il terminale o esegui:
           source ~/.zshrc

      Per verificare che tutto funzioni:
           night-agent doctor
    EOS
  end

  test do
    # Test che il binario risponde correttamente
    output = shell_output("#{bin}/night-agent --help")
    assert_match "night-agent", output

    # Test che la policy di default è accessibile
    assert_predicate pkgshare / "configs" / "default_policy.yaml", :exist?
  end
end
