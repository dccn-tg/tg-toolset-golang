# build with the following command:
# rpmbuild -bb
%define debug_package %{nil}

Name:       tg-toolset
Version:    %{getenv:VERSION}
Release:    1%{?dist}
Summary:    CLI tools for interacting various services managed by the TG.
License:    FIXME
URL: https://github.com/Donders-Institute/%{name}-golang
Source0: https://github.com/Donders-Institute/%{name}-golang/archive/%{version}.tar.gz

BuildArch: x86_64
#BuildRequires: systemd

# defin the GOPATH that is created later within the extracted source code.
%define gopath %{_tmppath}/go.rpmbuild-%{name}-%{version}

%description
CLI tools for interacting various services managed by the TG.

%prep
%setup -q

%build
# create GOPATH structure within the source code
mkdir -p %{gopath}
# copy entire directory into gopath, this duplicate the source code
cd %{buildroot}; GOPATH=%{gopath} make

%install
mkdir -p %{buildroot}/%{_bindir}
#mkdir -p %{buildroot}/%{_sysconfdir}/bash_completion.d
## install files for client tools
install -m 755 %{gopath}/bin/repoutil %{buildroot}/%{_bindir}/repoutil
install -m 755 %{gopath}/bin/prj_setacl %{buildroot}/%{_bindir}/prj_setacl
install -m 755 %{gopath}/bin/prj_getacl %{buildroot}/%{_bindir}/prj_getacl
install -m 755 %{gopath}/bin/prj_delacl %{buildroot}/%{_bindir}/prj_delacl
#install -m 644 %{gopath}/src/github.com/Donders-Institute/%{name}/hpcutil %{buildroot}/%{_sysconfdir}/bash_completion.d/hpcutil

%files
%{_bindir}/repoutil
%{_bindir}/prj_setacl
%{_bindir}/prj_getacl
%{_bindir}/prj_delacl
#%{_sysconfdir}/bash_completion.d/hpcutil

%clean
rm -rf %{gopath} 
rm -f %{_topdir}/SOURCES/%{version}.tar.gz
rm -rf $RPM_BUILD_ROOT

%changelog
* Thu Jun 23 2020 Hong Lee <h.lee@donders.ru.nl> - 0.1
- first rpmbuild implementation
