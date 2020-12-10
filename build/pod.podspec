Pod::Spec.new do |spec|
  spec.name         = 'Gech'
  spec.version      = '{{.Version}}'
  spec.license      = { :type => 'GNU Lesser General Public License, Version 3.0' }
  spec.homepage     = 'https://github.com/etvchaineum/go-etvchaineum'
  spec.authors      = { {{range .Contributors}}
		'{{.Name}}' => '{{.Email}}',{{end}}
	}
  spec.summary      = 'iOS Etvchain Client'
  spec.source       = { :git => 'https://github.com/etvchaineum/go-etvchaineum.git', :commit => '{{.Commit}}' }

	spec.platform = :ios
  spec.ios.deployment_target  = '9.0'
	spec.ios.vendored_frameworks = 'Frameworks/Gech.framework'

	spec.prepare_command = <<-CMD
    curl https://gechstore.blob.core.windows.net/builds/{{.Archive}}.tar.gz | tar -xvz
    mkdir Frameworks
    mv {{.Archive}}/Gech.framework Frameworks
    rm -rf {{.Archive}}
  CMD
end
