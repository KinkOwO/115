// holder_sub_14450AB40_0x14450ab40

unsigned __int64 __fastcall sub_14450AB40(__int64 a1)
{
  unsigned __int64 result; // rax
  unsigned __int64 v3; // rbp
  unsigned int v4; // ebx
  __int64 v5; // rax
  unsigned int v6; // r14d
  int v7; // edx
  _QWORD **v8; // r8
  _QWORD *v9; // rcx
  _QWORD *v10; // rcx
  _QWORD *v11; // r15
  __int64 v12; // rdx
  __int64 v13; // rax
  __int64 v14; // r8
  _QWORD *v15; // r9
  _QWORD *v16; // rax
  __int64 v17; // rcx
  __int64 v18; // rcx
  _QWORD *v19; // rax
  char v20; // di
  int v21; // esi
  _QWORD *v22; // rbx
  __int64 v23; // rax
  __int64 v24; // rcx
  _BOOL8 v25; // rdx
  __int64 v26; // rax
  __int64 v27; // rcx
  __int64 v28; // rax
  void (__fastcall ***v29)(_QWORD); // rcx
  __int64 v30; // rax
  __int64 v31; // rax
  __int64 v32; // rcx
  unsigned __int64 v33; // rbx
  __int64 v34; // r9
  int v35; // r8d
  _QWORD *v36; // r10
  _QWORD *v37; // rcx
  __int64 v38; // rdx
  __int64 v39; // rax
  __int64 v40; // rax
  __int64 v41; // r9
  __int64 v42; // rcx

  result = sub_1459AB090(qword_14E66C090);
  if ( (_BYTE)result )
  {
    result = sub_145F0BA60(qword_14E683C08);
    if ( result )
    {
      result = sub_145EFFF10(result);
      v3 = result;
      if ( result )
      {
        v4 = *(_DWORD *)(sub_140740290() + 188);
        v5 = sub_140740290();
        result = sub_140740320(v5, v4);
        v6 = result;
        v7 = 0;
        v8 = *(_QWORD ***)(v3 + 24);
        v9 = *v8;
        if ( *v8 != v8 )
        {
          while ( v7 != (_DWORD)result )
          {
            v9 = (_QWORD *)*v9;
            ++v7;
            if ( v9 == v8 )
              return result;
          }
          v10 = v9 + 2;
          if ( v10 )
          {
            v11 = v10 + 224;
            result = v10[225];
            if ( result )
            {
              if ( *(_DWORD *)(result + 8) && v10[226] )
              {
                sub_1401F10B0(&qword_14EF2CA80, v10 + 224);
                if ( !qword_14EF2CA88 || (v13 = qword_14EF2CA90, !*(_DWORD *)(qword_14EF2CA88 + 8)) )
                  v13 = 0;
                v14 = v13 - 48;
                if ( !v13 )
                  v14 = 0;
                v15 = *(_QWORD **)(v3 + 24);
                v16 = (_QWORD *)*v15;
                if ( (_QWORD *)*v15 != v15 )
                {
                  while ( 1 )
                  {
                    v12 = v14 + 48;
                    if ( !v14 )
                      v12 = 0;
                    v17 = v16[227];
                    if ( v17 && *(_DWORD *)(v17 + 8) )
                      v18 = v16[228];
                    else
                      v18 = 0;
                    if ( v18 == v12 )
                      break;
                    v16 = (_QWORD *)*v16;
                    if ( v16 == v15 )
                      goto LABEL_38;
                  }
                  v19 = v16 + 2;
                  if ( v19 )
                  {
                    v20 = *((_BYTE *)v19 + 1853);
                    v21 = *(_DWORD *)v19;
                    *(_DWORD *)(v3 + 40) = *(_DWORD *)v19;
                    *(_BYTE *)(v3 + 48) = v20;
                    v22 = (_QWORD *)*v15;
                    if ( (_QWORD *)*v15 != v15 )
                    {
                      do
                      {
                        v23 = v22[227];
                        if ( v23 )
                        {
                          if ( *(_DWORD *)(v23 + 8) )
                          {
                            v24 = v22[228];
                            if ( v24 )
                            {
                              v25 = *((_DWORD *)v22 + 4) != v21 || *((_BYTE *)v22 + 1869) != v20;
                              (*(void (__fastcall **)(__int64, _BOOL8))(*(_QWORD *)(v24 - 48) + 9072LL))(v24 - 48, v25);
                            }
                          }
                        }
                        v22 = (_QWORD *)*v22;
                      }
                      while ( v22 != *(_QWORD **)(v3 + 24) );
                    }
                  }
                }
LABEL_38:
                sub_14450DA20(a1, v12, v14);
                v26 = sub_140413AB0();
                sub_144DBE720(v26);
                v27 = qword_14E63AE60;
                if ( !qword_14E63AE60 )
                {
                  v28 = sub_146E8BA20(336);
                  if ( v28 )
                    v29 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v28);
                  else
                    v29 = 0;
                  qword_14E63AE60 = (__int64)v29;
                  (**v29)(v29);
                  v27 = qword_14E63AE60;
                }
                sub_1447EE520(v27, v6);
                v30 = sub_1401DEF50(v11);
                (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v30 + 4816LL))(v30, 0);
                sub_144503430(a1, v6);
                sub_14450E650(a1);
                v31 = sub_145EFAFB0();
                v32 = v31 + 48;
                if ( !v31 )
                  v32 = 0;
                if ( !qword_14EF2CA88 || (result = qword_14EF2CA90, !*(_DWORD *)(qword_14EF2CA88 + 8)) )
                  result = 0;
                if ( result == v32 )
                {
                  result = sub_145F0BA60(qword_14E683C08);
                  if ( result )
                  {
                    result = sub_145EFFF10(result);
                    v33 = result;
                    if ( result )
                    {
                      if ( *(_QWORD *)(result + 32) > 1u )
                      {
                        v34 = sub_145EFAFB0();
                        v35 = 0;
                        v36 = *(_QWORD **)(v33 + 24);
                        v37 = (_QWORD *)*v36;
                        if ( (_QWORD *)*v36 == v36 )
                        {
LABEL_61:
                          v35 = -1;
                        }
                        else
                        {
                          while ( 1 )
                          {
                            v38 = v34 + 48;
                            if ( !v34 )
                              v38 = 0;
                            v39 = v37[227];
                            if ( v39 && *(_DWORD *)(v39 + 8) )
                              v40 = v37[228];
                            else
                              v40 = 0;
                            if ( v40 == v38 )
                              break;
                            v37 = (_QWORD *)*v37;
                            ++v35;
                            if ( v37 == v36 )
                              goto LABEL_61;
                          }
                        }
                        v41 = *(_QWORD *)(a1 + 888);
                        v42 = *(_QWORD *)(a1 + 896) - v41;
                        result = (unsigned __int64)((unsigned __int128)(v42 * (__int128)0x4924924924924925LL) >> 64) >> 63;
                        if ( v35 < (unsigned __int64)(v42 / 28) )
                          return sub_145EEAA70(v41 + 28LL * v35);
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
  return result;
}

