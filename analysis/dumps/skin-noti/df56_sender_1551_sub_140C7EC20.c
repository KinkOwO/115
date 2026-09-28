// sender_1551_sub_140C7EC20

__int64 __fastcall sub_140C7EC20(__int64 a1)
{
  __int64 v1; // rax
  __int64 result; // rax
  __int64 v3; // rdi
  __int64 v4; // rbx
  __int64 v5; // rax
  __int64 v6; // rbx
  __int64 v7; // rax
  __int64 v8; // rbx
  _QWORD *v9; // rax
  const char *v10; // rsi
  __int64 v11; // rbx
  int v12; // edi
  int v13; // ecx
  int v14; // edx
  __int64 v15; // rax
  int v16; // eax
  __int64 v17; // rax
  __int64 v18; // rax
  __int64 v19; // rcx
  __int64 v20; // rax
  __int64 v21; // rcx
  __int64 v22; // rax
  __int64 v23; // r8
  __int64 v24; // rdx
  __int64 v25; // rcx
  __int64 v26; // rax
  __int64 v27; // rax
  unsigned __int8 v28; // si
  __int64 v29; // rdi
  __int64 v30; // rcx
  __int64 v31; // rax
  __int64 v32; // rcx
  __int64 v33; // rbx
  __int64 v34; // rax
  unsigned int v35; // eax
  __int64 v36; // rcx
  __int64 v37; // rax
  __int64 v38; // rcx
  __int64 v39; // rax
  __int64 v40; // rdx
  __int64 v41; // rcx
  __int64 v42; // rax
  __int64 v43; // rcx
  __int64 v44; // rax
  __int64 v45; // rcx
  __int64 v46; // rax
  __int64 v47; // rcx
  __int64 v48; // rax
  __int64 v49; // rdx
  __int64 v50; // rcx
  __int64 v51; // [rsp+30h] [rbp-58h] BYREF
  _BYTE v52[24]; // [rsp+38h] [rbp-50h] BYREF
  _BYTE v53[40]; // [rsp+50h] [rbp-38h] BYREF
  void *retaddr; // [rsp+88h] [rbp+0h]

  v1 = sub_146C82A50(a1);
  result = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v1 + 304LL))(v1);
  v3 = result;
  if ( result )
  {
    result = sub_1459A9080(qword_14E66C090);
    v4 = result;
    if ( result )
    {
      result = sub_145B349F0(result);
      if ( !(_BYTE)result )
      {
        v5 = sub_145B2F1E0(v4, v3);
        v6 = v5;
        if ( v5 )
        {
          if ( sub_144D24C10(v5) )
          {
            if ( *(_BYTE *)(sub_144D24C10(v6) + 3888) )
            {
              v7 = sub_145F0BA60(qword_14E683C08);
              v8 = v7;
              if ( v7 )
              {
                v9 = (_QWORD *)sub_140176170(v7);
                v10 = (const char *)sub_146E90780(*v9);
                v11 = sub_140176170(v8);
                sub_146E920A0(v11 + 36, &v51);
                v12 = v51;
                v13 = *(_DWORD *)(v11 + 40);
                v14 = *(_DWORD *)(v11 + 36) + v51 + 196;
                if ( v13 && v14 && v13 != v14 && retaddr )
                {
                  sub_146D89B40(retaddr, v11 + 36);
                  v12 = v51;
                }
                v15 = sub_145AD9A90(qword_14E683C80);
                v16 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v15 + 224LL))(v15);
                sub_14012F0D0(v53, "%20s%2d%12d", v10, v12, v16);
                v17 = sub_145EFAFB0();
                v18 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v17 + 7656LL))(v17);
                sub_146E920B0(v18 + 96, &v51);
                sub_14012F0D0(v52, "%lld", v51);
                v20 = sub_146D74000(v19);
                sub_146D746E0(v20, 627);
                v22 = sub_146D74000(v21);
                v23 = -1;
                do
                  ++v23;
                while ( v52[v23] );
                sub_146D75C50(v22, v52);
                sub_146D75AF0(v25, v24);
              }
            }
          }
        }
        v26 = sub_140C6ED80();
        result = sub_144AD16D0(v26);
        if ( (_BYTE)result )
        {
          result = sub_145EFAFB0();
          if ( result )
          {
            v27 = sub_145EFAFB0();
            v28 = sub_145F13470(qword_14E683C20, v27);
            v29 = sub_14021BE90(qword_14E683B28, 400015, 0);
            v31 = sub_146D74000(v30);
            sub_146D746E0(v31, 615);
            v33 = sub_146D74000(v32);
            v34 = sub_140C6ED80();
            v35 = sub_140696250(v34);
            sub_146D75CE0(v33, v35);
            v37 = sub_146D74000(v36);
            sub_146D75CE0(v37, 12);
            v39 = sub_146D74000(v38);
            LOBYTE(v40) = 1;
            sub_146D75CC0(v39, v40);
            v42 = sub_146D74000(v41);
            sub_146D76180(v42, 1);
            v44 = sub_146D74000(v43);
            sub_146D75CE0(v44, 400015);
            v46 = sub_146D74000(v45);
            sub_146D75CE0(v46, *(unsigned int *)(v29 + 2248));
            v48 = sub_146D74000(v47);
            sub_146D75CC0(v48, v28);
            return sub_146D75AF0(v50, v49);
          }
        }
      }
    }
  }
  return result;
}

