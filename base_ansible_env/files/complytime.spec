Name:           complytime
Version:        0.0.2
Release:        1%{?dist}
Summary:        ComplyTime leverages OSCAL to perform compliance assessment activities, using plugins for each stage of the lifecycle.

License:        Apache-2.0
URL:            https://github.com/complytime/complytime
Source0:        https://github.com/complytime/complytime/archive/refs/tags/v0.0.2.tar.gz

BuildRequires:  golang
BuildRequires:  make

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

%install
mkdir -p %{buildroot}%{_bindir}
install -m 0755 bin/complytime %{buildroot}%{_bindir}/complytime
mkdir -p %{buildroot}%{_libexecdir}/%{name}/plugins
install -m 0755 bin/openscap-plugin %{buildroot}%{_libexecdir}/%{name}/plugins/openscap-plugin
mkdir -p %{buildroot}%{_datadir}/%{name}
cp -rf docs/samples %{buildroot}%{_datadir}/%{name}
# TODO: Manifest file, related issue: CPLYTM-682
mkdir -p %{buildroot}%{_datadir}/%{name}/plugins

%check
make test-unit

%files
%license LICENSE
%{_bindir}/complytime
%doc README.md
%defattr(-,root,root,644)
%{_datadir}/%{name}

%files          openscap-plugin
%{_libexecdir}/%{name}/plugins/openscap-plugin
%doc cmd/openscap-plugin/README.md

%changelog
* Fri Apr 11 2025 Qingmin Duanmu <qduanmu@redhat.com>
- Separate package for openscap-plugin

* Tue Apr 08 2025 Marcus Burghardt <maburgha@redhat.com>
- Initial RPM
