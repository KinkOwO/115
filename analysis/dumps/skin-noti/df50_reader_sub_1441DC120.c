// reader_sub_1441DC120

__int64 __fastcall sub_1441DC120(__int64 a1)
{
  __int64 v2; // rcx
  __int64 v3; // rcx
  __int64 v4; // rcx
  __int64 v5; // rcx
  __int64 v6; // rcx
  __int64 v7; // rcx
  __int64 v8; // rcx
  __int64 v9; // rcx
  __int64 v10; // rcx
  __int64 v11; // rdx
  __int64 result; // rax
  __int64 v13; // rax
  __int64 v14; // rdx
  __int64 v15; // r12
  __int64 v16; // rcx
  __int64 v17; // rax
  __int64 v18; // r8
  int v19; // r14d
  _QWORD *v20; // rsi
  int v21; // ebx
  int v22; // edi
  int v23; // edx
  int v24; // r8d
  unsigned __int64 v25; // rdx
  __int64 v26; // rcx
  __int64 v27; // rcx
  __int64 v28; // rax
  int v29; // r14d
  _QWORD *v30; // rsi
  int v31; // ebx
  int v32; // edi
  int v33; // edx
  int v34; // r8d
  unsigned __int64 v35; // rdx
  __int64 v36; // rcx
  _BYTE v37[8]; // [rsp+68h] [rbp-29h] BYREF
  _BYTE v38[8]; // [rsp+70h] [rbp-21h] BYREF
  __int64 v39; // [rsp+78h] [rbp-19h]
  _QWORD v40[2]; // [rsp+80h] [rbp-11h] BYREF
  __int64 v41; // [rsp+90h] [rbp-1h]
  unsigned __int64 v42; // [rsp+98h] [rbp+7h]

  v39 = -2;
  v2 = *(_QWORD *)(a1 + 80);
  if ( !v2 || !(unsigned __int8)sub_146ED0010(v2) )
  {
    v3 = *(_QWORD *)(a1 + 224);
    if ( !v3 || !(unsigned __int8)sub_146ED0010(v3) )
    {
      v4 = *(_QWORD *)(a1 + 192);
      if ( !v4 || !(unsigned __int8)sub_146ED0010(v4) )
      {
        v5 = *(_QWORD *)(a1 + 8);
        if ( !v5 || !(unsigned __int8)sub_146ED0010(v5) )
        {
          v6 = *(_QWORD *)(a1 + 208);
          if ( !v6 || !(unsigned __int8)sub_146ED0010(v6) )
          {
            v7 = *(_QWORD *)(a1 + 128);
            if ( !v7 || !(unsigned __int8)sub_146ED0010(v7) )
            {
              v8 = *(_QWORD *)(a1 + 96);
              if ( !v8 || !(unsigned __int8)sub_146ED0010(v8) )
              {
                v9 = *(_QWORD *)(a1 + 304);
                if ( !v9 || !(unsigned __int8)sub_146ED0010(v9) )
                  goto LABEL_20;
              }
            }
          }
        }
      }
    }
  }
  v10 = *(_QWORD *)(a1 + 272);
  if ( v10 && !(unsigned __int8)sub_141FB6530(v10) )
    LOBYTE(v11) = 1;
  else
LABEL_20:
    v11 = 0;
  (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 256) + 16LL))(*(_QWORD *)(a1 + 256), v11);
  result = sub_14501B3E0(*(_QWORD *)(a1 + 64));
  if ( result )
  {
    v13 = sub_14501B3E0(*(_QWORD *)(a1 + 64));
    if ( (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)v13 + 408LL))(v13) )
      v14 = 0;
    else
      LOBYTE(v14) = 1;
    result = (*(__int64 (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 224) + 24LL))(*(_QWORD *)(a1 + 224), v14);
  }
  v15 = -1;
  if ( *(_BYTE *)(a1 + 321) )
  {
    v16 = *(_QWORD *)(a1 + 128);
    if ( v16 )
    {
      result = sub_146ED0010(v16);
      if ( (_BYTE)result )
      {
        v17 = sub_146EF69A0(*(_QWORD *)(a1 + 128));
        v40[0] = 0;
        v41 = 0;
        v42 = 7;
        v18 = -1;
        do
          ++v18;
        while ( *(_WORD *)(v17 + 2 * v18) );
        sub_14014C8D0(v40, v17);
        v19 = sub_1429BDDE0(qword_14E683C78);
        v20 = v40;
        if ( v42 >= 8 )
          v20 = (_QWORD *)v40[0];
        v21 = dword_14F1C0788;
        v22 = dword_14F1C0880;
        sub_146EC9E00(*(_QWORD *)(a1 + 128), v37);
        sub_146EC9E00(*(_QWORD *)(a1 + 128), v38);
        result = sub_145561F90(v19, v23, v24, v22, v21, 1, (__int64)v20, 0, 0, 1153957888, 0, 0);
        if ( v42 >= 8 )
        {
          v25 = 2 * v42 + 2;
          v26 = v40[0];
          if ( v25 >= 0x1000 )
          {
            v25 = 2 * v42 + 41;
            v26 = *(_QWORD *)(v40[0] - 8LL);
            if ( (unsigned __int64)(v40[0] - v26 - 8) > 0x1F )
              sub_148AAF304(v26, v25);
          }
          result = sub_146E9F3A0(v26, v25);
        }
        v41 = 0;
        v42 = 7;
        LOWORD(v40[0]) = 0;
      }
    }
  }
  if ( *(_BYTE *)(a1 + 320) )
  {
    v27 = *(_QWORD *)(a1 + 96);
    if ( v27 )
    {
      result = sub_146ED0010(v27);
      if ( (_BYTE)result )
      {
        v28 = sub_146EF69A0(*(_QWORD *)(a1 + 96));
        v40[0] = 0;
        v41 = 0;
        v42 = 7;
        do
          ++v15;
        while ( *(_WORD *)(v28 + 2 * v15) );
        sub_14014C8D0(v40, v28);
        v29 = sub_1429BDDE0(qword_14E683C78);
        v30 = v40;
        if ( v42 >= 8 )
          v30 = (_QWORD *)v40[0];
        v31 = dword_14F1C0788;
        v32 = dword_14F1C0880;
        sub_146EC9E00(*(_QWORD *)(a1 + 96), v38);
        sub_146EC9E00(*(_QWORD *)(a1 + 96), v37);
        result = sub_145561F90(v29, v33, v34, v32, v31, 1, (__int64)v30, 0, 0, 1153957888, 0, 0);
        if ( v42 >= 8 )
        {
          v35 = 2 * v42 + 2;
          v36 = v40[0];
          if ( v35 >= 0x1000 )
          {
            v35 = 2 * v42 + 41;
            v36 = *(_QWORD *)(v40[0] - 8LL);
            if ( (unsigned __int64)(v40[0] - v36 - 8) > 0x1F )
              sub_148AAF304(v36, v35);
          }
          result = sub_146E9F3A0(v36, v35);
        }
        v41 = 0;
        v42 = 7;
        LOWORD(v40[0]) = 0;
      }
    }
  }
  return result;
}

