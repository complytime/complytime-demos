Name:           complytime
Version:        0.0.5
Release:        1%{?dist}
Summary:        ComplyTime leverages OSCAL to perform compliance assessment activities, using plugins for each stage of the lifecycle.

License:        Apache-2.0
URL:            https://github.com/complytime/complytime
Source0:        https://github.com/complytime/complytime/archive/refs/tags/v0.0.5.tar.gz

BuildRequires:  golang
BuildRequires:  make
BuildRequires:  pandoc

%description
ComplyTime leverages OSCAL to perform compliance assessment activities, using plugins for each stage of the lifecycle.

%package        openscap-plugin
Summary:        A plugin which extends the ComplyTime capabilities to use OpenSCAP
Requires:       %{name}%{?_isa} = %{version}-%{release}
Requires:       scap-security-guide
%description    openscap-plugin
openscap-plugin is a plugin which extends the ComplyTime capabilities to use OpenSCAP. The plugin communicates
with ComplyTime via gRPC, providing a standard and consistent communication mechanism that gives independence
for plugin developers to choose their preferred languages.

%prep
%setup -q

%undefine _missing_build_ids_terminate_build
%undefine _debugsource_packages

%build
make build
make man

%install
# Install ComplyTime directories
install -d %{buildroot}%{_bindir}
install -d -m 0755 %{buildroot}%{_datadir}/%{name}/{plugins,bundles,controls}
install -d -m 0755 %{buildroot}%{_libexecdir}/%{name}/plugins
install -d -m 0755 %{buildroot}%{_sysconfdir}/%{name}/config.d
install -d -m 0755 %{buildroot}%{_mandir}/{man1,man5}

# Copy sample data to be consumed by ComplyTime CLI
cp -r docs/samples %{buildroot}%{_datadir}/%{name}

# Install files for ComplyTime CLI
install -m 0755 bin/complytime %{buildroot}%{_bindir}/complytime
install -m 0644 docs/man/complytime.1 %{buildroot}%{_mandir}/man1/complytime.1

# Install files for openscap-plugin package
install -m 0755 bin/openscap-plugin %{buildroot}%{_libexecdir}/%{name}/plugins/openscap-plugin
install -m 0644 docs/man/c2p-openscap-manifest.5 %{buildroot}%{_mandir}/man5/c2p-openscap-manifest.5

%check
make test-unit

%files
%defattr(0644, root, root, 0755)
%attr(0755, root, root) %{_bindir}/complytime
%license LICENSE
%doc %{_mandir}/man1/complytime.1*
%{_datadir}/%{name}/samples/{sample-catalog.json,sample-component-definition.json,sample-profile.json}
%{_datadir}/%{name}/{plugins,bundles,controls}
%{_sysconfdir}/%{name}/config.d

%files          openscap-plugin
%attr(0755, root, root) %{_libexecdir}/%{name}/plugins/openscap-plugin
%doc %{_mandir}/man5/c2p-openscap-manifest.5*

%changelog
* Tue May 6 2025 Qingmin Duanmu <qduanmu@redhat.com>
- Add complytime and openscap plugin man pages

* Wed Apr 30 2025 Qingmin Duanmu <qduanmu@redhat.com>
- Separate plugin binary from manifest

* Fri Apr 11 2025 Qingmin Duanmu <qduanmu@redhat.com>
- Separate package for openscap-plugin

* Tue Apr 08 2025 Marcus Burghardt <maburgha@redhat.com>
- Initial RPM
