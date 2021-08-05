package melcloud

type loginRequest struct {
    AppVersion, Email, Password string
    CaptchaResponse *string
    Language int
    Persist bool
}

type loginResponse struct {
    ErrorId *int
    LoginData *struct {
        ContextKey, Name string
    }
}
