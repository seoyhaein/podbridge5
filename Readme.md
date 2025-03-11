## TODO
- container.go 테스트 파일 작성  
~~- container 생성까지 작성. 테스트 파일 작성 필요.~~ (성공) 
- policy.json 과 REGISTRIES.CONF 디폴트로 잡았는데 개발을 위해서 이거 세부적으로 잡아주어야 함. 
~~- healthcheck 구문 테스트 필요. shellscript 집어넣어야 함.~~  
~~- defer 구문 기억해내자.~~  
- buildah 관련해서 buildah 도 필요한지 살펴본다. image.go 같은 경우는 이미지 빌드에 관련된 부분이라서 buildah 를 활용해야 한다. 
- volume 관련해서는 notion 확인하고 진행하자.  
~~- 이거 완료되면 podbridge 에 통합할 예정임.  v4 폴더와 v5 폴더 만들어서 적용함. 시간날때 해두자.~~  
- Run 메서드 여러개 돌릴때 문제될 수 있음. 컨테이너 여러개 만들때 문제될 수 있음. Run 은 빨리 종료시켜야함.
~~- healthcheck 는 goroutine 으로 만들어 두고, 이것을 모니터링 하는 것도 goroutine 으로 하는 것이 좋을 것 같다.~~  
~~- healthcheck 같은 경우는 각 컨테이너의 상태를 확인할 수 있는 모니터링 메서드를 하나 만들어서 여기서 관리하도록 하는 방향으로 간다.~~  
- Run 메서드는 바로 실행 종료 할 수 있도록 
~~- healthcheck.sh 최적화 시킨고 문서화 한다.~~ 
- 문서화는 별도로 진행한다.
- image.go 정리하고 문서화해 놓는다.
~~- chain 형태로 메서드를 연결해서 사용하는 방식으로 했는데 이렇게 하지 말고 오류가 발생했을때 명확히 알 수 있는 형태로 하자.~~  
- 시간 제한을 거는 문제 구현 해야함.
~~- heathcheck_new.sh 로 해서 테스트 해보고 수동으로 했을때는 정상작동하는데 테스트 할때 않되는 이유 찾자.~~

## 생각하기
~~- 지금 2초 후에 헬스체크를 ㅎ고 있는데 이럴 경우 2초 보다 일찍 끝나는 것은 헬스체크를 하지 않는다.~~  
~~- 그리고 이러한 시간은 일단 고정으로 잡아 두었다.~~
~~- 버그 해결해야함.~~
~~- healthcheck 같은 경우는 여러번 반복적으로 실행 될 수 있다. 이거 유념해서 코드를 다시 살펴봐야 한다.~~ 
~~- 그리고 이건 빠르게 실행해야 하는 부분이다.~~
~~- 에러를 어떻게 처리하는지 에러 발생하는 script 를 만들어 줘서 하나 하나 다 테스트 해야하고 이상한 125 에러 나오던데 이거 뭔지 파악해야 한다.~~ 
## image.go
~~- 기본 메서드 와 여기서 healthcheck.sh 를 넣는 버전과 사용자의 dockerfile 을 받아서 이미지 만들어주는 것.~~  
~~- 사용자 이미지에서 healthcheck.sh 를 넣어서 이미지를 만들어 주는 것 필요.~~  
~~- healthcheck.sh 등을 넣어서 만들어준 이미지는 내부에서만 사용되는 이미지임.(영업비밀. notion 참고.)~~  
- etcd conf 확인해서, podman 살아있는지 죽었는지 확인하고 죽으면 살리는 루틴 생각해보자.(진행중)
~~- storage 관련 conf 파일 작성해주거나 작성 루틴 만들어서 podman 오류 없애야 함.~~  또 에러남. 젠장.
~~- 일단 buildah version 과 podman info 에서 나오는 버전을 맞추자. buildah 버전을 맞춰서 재설치 하자.~~  
- ~~CreateDefaultImage~~ CreateImageWithDockerfile 수정해야 함. alpine 으로 했을때는 Dockerfile.alpine.executor 와 동일 해야 함.
~~- 이미지를 만들때 CMD ["/bin/sh", "-c", "/app/executor.sh"] 이런 식으로 만들어 주어야 함.~~ 
- 주요한 테스트가 끝나면 db 에 넣는 것을 생각 해야함.  

## container.go
- 런할때는 좀더 생각해야 함. 떨어지는 이미지나 필요한 이미지를 넣어주는 것을 생각해야 함.  

## 컨테이너 테스트 
- ubuntu, centos 및 기타 다른 os 로 테스트 진행
- healthcheck.sh 권한 설정 빠져 있음.
- 클러스터나 작업하는 노드가 완전 폐쇄형일 경우 Dockerfile 구성을 달리 해야함. (대단히 중요. 이 경우 개방형과 폐쇄형 둘다 구분해서 만들어 줘야 함.)  
- executor.sh, healthcheck.sh 같은 경우는 외부에 노출 시키지 않고 이런 것들이 들어간 이미지 역시 외부 노출 시키지 않는다.
- 별도의 레지스트리는 두지만 여기에 들어가는 것은 사용자 이미지 이지 내부적으로 쓰이는 이미지(위에서 언급한 이미지)는 아니다.  
- error 코드 정리 해야함.  
- executor.go 수정해 주어야 함.
~~- executor.sh 코드 확인하자. 테스트 진행하자.~~
~~- executor.sh, healthcheck.sh 코드 단순화 install.sh 하나 만들어 줌.~~  
- 메서드들을 통일성있게 구성하자. config.methodA 이런식으로 환경설정이 필요한 경우는 이렇게 구성하자.
## 확인하자.
https://github.com/containers/buildah/blob/main/docs/tutorials/04-include-in-your-build-tool.md

```
Supplying defaults for Run()
If you need to run a command as part of the build, you'll have to dig up a couple of defaults that aren't picked up automatically:

conf, err := config.Default()
capabilitiesForRoot, err := conf.Capabilities("root", nil, nil)
isolation, err := parse.IsolationOption("")
```