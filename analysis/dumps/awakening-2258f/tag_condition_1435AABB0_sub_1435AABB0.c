// sub_1435AABB0  va=0x1435AABB0  size=1556

__int64 __fastcall sub_1435AABB0(__int64 a1, __int64 a2, __int64 a3)
{
  char *v6; // rax
  __int64 v7; // rcx
  char *v8; // r9
  __int64 v9; // r8
  unsigned __int16 v10; // cx
  signed __int64 v11; // r9
  bool v12; // cc
  unsigned __int16 v13; // cx
  __int64 v14; // rax
  __int64 v15; // rcx
  unsigned __int16 *v16; // r9
  __int64 v17; // r8
  unsigned __int16 v18; // cx
  bool v19; // cc
  unsigned __int16 v20; // cx
  __int64 v21; // rax
  __int64 v22; // rcx
  unsigned __int16 *v23; // r9
  __int64 v24; // r8
  unsigned __int16 v25; // cx
  bool v26; // cc
  unsigned __int16 v27; // cx
  __int64 v28; // rax
  __int64 v29; // rcx
  unsigned __int16 *v30; // r9
  __int64 v31; // r8
  unsigned __int16 v32; // cx
  bool v33; // cc
  unsigned __int16 v34; // cx
  __int64 v35; // rax
  __int64 v36; // rcx
  unsigned __int16 *v37; // r9
  __int64 v38; // r8
  unsigned __int16 v39; // cx
  bool v40; // cc
  unsigned __int16 v41; // cx
  char *v42; // rax
  __int64 v43; // rcx
  char *v44; // r9
  __int64 v45; // r8
  unsigned __int16 v46; // cx
  signed __int64 v47; // r9
  bool v48; // cc
  unsigned __int16 v49; // cx
  char v50; // al
  __int64 v51; // r8
  __int64 v52; // r8
  __int64 v53; // r9
  __int64 v54; // r8
  __int64 v55; // r9
  char *v56; // rax
  __int64 v57; // rcx
  char *v58; // r9
  unsigned __int16 v59; // cx
  signed __int64 v60; // r9
  bool v61; // cc
  unsigned __int16 v62; // cx
  unsigned __int8 v63; // bl
  unsigned __int64 v64; // rdx
  __int64 v65; // rcx
  unsigned __int64 v66; // rdx
  __int64 v67; // rcx
  int i; // [rsp+20h] [rbp-49h] BYREF
  __int128 v70; // [rsp+28h] [rbp-41h] BYREF
  __int64 v71; // [rsp+38h] [rbp-31h]
  __int64 v72; // [rsp+40h] [rbp-29h]
  __int64 v73; // [rsp+48h] [rbp-21h]
  _QWORD v74[2]; // [rsp+50h] [rbp-19h] BYREF
  __int64 v75; // [rsp+60h] [rbp-9h]
  unsigned __int64 v76; // [rsp+68h] [rbp-1h]
  _QWORD v77[2]; // [rsp+70h] [rbp+7h] BYREF
  __int64 v78; // [rsp+80h] [rbp+17h]
  unsigned __int64 v79; // [rsp+88h] [rbp+1Fh]

  v72 = -2;
  v74[0] = 0;
  v75 = 0;
  v76 = 7;
  sub_14014C8D0(v74, (void *)&Source);
  __wind
  {
    v77[0] = 0;
    v78 = 0;
    v79 = 7;
    sub_14014C8D0(v77, (void *)&Source);
    __wind
    {
      for ( i = 0; ; *(_DWORD *)(a3 + 4) = i )
      {
        while ( 1 )
        {
          LOBYTE(v51) = 1;
          if ( (unsigned __int8)sub_1470A1A80(a2, v74, v51) == 0 )
          {
LABEL_101:
            v63 = 0;
            goto LABEL_116;
          }
          v6 = (char *)sub_146E8C7D0(&unk_149E4B778);
          v7 = -1;
          do
            ++v7;
          while ( *(_WORD *)&v6[2 * v7] != 0 );
          v8 = (char *)v74;
          if ( v76 >= 8 )
            v8 = (char *)v74[0];
          v9 = v75;
          if ( v75 == v7 )
          {
            if ( v75 == 0 )
            {
LABEL_100:
              v63 = 1;
              goto LABEL_116;
            }
            v10 = *(_WORD *)v8;
            if ( *(_WORD *)v8 >= *(_WORD *)v6 )
            {
              v11 = v8 - v6;
              v12 = v10 <= *(_WORD *)v6;
              do
              {
                if ( !v12 )
                  break;
                if ( v9 == 1 )
                  goto LABEL_100;
                --v9;
                v6 += 2;
                v13 = *(_WORD *)&v6[v11];
                v12 = v13 <= *(_WORD *)v6;
              }
              while ( v13 >= *(_WORD *)v6 );
            }
          }
          v14 = sub_146E8C7D0(&unk_149C642C0);
          v15 = -1;
          do
            ++v15;
          while ( *(_WORD *)(v14 + 2 * v15) != 0 );
          v16 = (unsigned __int16 *)v74;
          if ( v76 >= 8 )
            v16 = (unsigned __int16 *)v74[0];
          v17 = v75;
          if ( v75 == v15 )
            break;
LABEL_25:
          v21 = sub_146E8C7D0(&unk_149E4B7A0);
          v22 = -1;
          do
            ++v22;
          while ( *(_WORD *)(v21 + 2 * v22) != 0 );
          v23 = (unsigned __int16 *)v74;
          if ( v76 >= 8 )
            v23 = (unsigned __int16 *)v74[0];
          v24 = v75;
          if ( v75 != v22 )
            goto LABEL_36;
          if ( v75 != 0 )
          {
            v25 = *v23;
            if ( *v23 >= *(_WORD *)v21 )
            {
              v23 = (unsigned __int16 *)((char *)v23 - v21);
              v26 = v25 <= *(_WORD *)v21;
              do
              {
                if ( !v26 )
                  break;
                if ( v24 == 1 )
                  goto LABEL_77;
                --v24;
                v21 += 2;
                v27 = *(unsigned __int16 *)((char *)v23 + v21);
                v26 = v27 <= *(_WORD *)v21;
              }
              while ( v27 >= *(_WORD *)v21 );
            }
LABEL_36:
            v28 = sub_146E8C7D0(&unk_149E4B7C0);
            v29 = -1;
            do
              ++v29;
            while ( *(_WORD *)(v28 + 2 * v29) != 0 );
            v30 = (unsigned __int16 *)v74;
            if ( v76 >= 8 )
              v30 = (unsigned __int16 *)v74[0];
            v31 = v75;
            if ( v75 != v29 )
              goto LABEL_47;
            if ( v75 != 0 )
            {
              v32 = *v30;
              if ( *v30 >= *(_WORD *)v28 )
              {
                v30 = (unsigned __int16 *)((char *)v30 - v28);
                v33 = v32 <= *(_WORD *)v28;
                do
                {
                  if ( !v33 )
                    break;
                  if ( v31 == 1 )
                    goto LABEL_80;
                  --v31;
                  v28 += 2;
                  v34 = *(unsigned __int16 *)((char *)v30 + v28);
                  v33 = v34 <= *(_WORD *)v28;
                }
                while ( v34 >= *(_WORD *)v28 );
              }
LABEL_47:
              v35 = sub_146E8C7D0(&unk_149684168);
              v36 = -1;
              do
                ++v36;
              while ( *(_WORD *)(v35 + 2 * v36) != 0 );
              v37 = (unsigned __int16 *)v74;
              if ( v76 >= 8 )
                v37 = (unsigned __int16 *)v74[0];
              v38 = v75;
              if ( v75 != v36 )
                goto LABEL_58;
              if ( v75 != 0 )
              {
                v39 = *v37;
                if ( *v37 >= *(_WORD *)v35 )
                {
                  v37 = (unsigned __int16 *)((char *)v37 - v35);
                  v40 = v39 <= *(_WORD *)v35;
                  do
                  {
                    if ( !v40 )
                      break;
                    if ( v38 == 1 )
                      goto LABEL_83;
                    --v38;
                    v35 += 2;
                    v41 = *(unsigned __int16 *)((char *)v37 + v35);
                    v40 = v41 <= *(_WORD *)v35;
                  }
                  while ( v41 >= *(_WORD *)v35 );
                }
LABEL_58:
                v42 = (char *)sub_146E8C7D0(&unk_149E4B7E8);
                v43 = -1;
                do
                  ++v43;
                while ( *(_WORD *)&v42[2 * v43] != 0 );
                v44 = (char *)v74;
                if ( v76 >= 8 )
                  v44 = (char *)v74[0];
                v45 = v75;
                if ( v75 == v43 )
                {
                  if ( v75 == 0 )
                  {
LABEL_84:
                    v50 = 1;
                    goto LABEL_70;
                  }
                  v46 = *(_WORD *)v44;
                  if ( *(_WORD *)v44 >= *(_WORD *)v42 )
                  {
                    v47 = v44 - v42;
                    v48 = v46 <= *(_WORD *)v42;
                    do
                    {
                      if ( !v48 )
                        break;
                      if ( v45 == 1 )
                        goto LABEL_84;
                      --v45;
                      v42 += 2;
                      v49 = *(_WORD *)&v42[v47];
                      v48 = v49 <= *(_WORD *)v42;
                    }
                    while ( v49 >= *(_WORD *)v42 );
                  }
                }
                v50 = 0;
LABEL_70:
                if ( v50 != 0 )
                {
                  v70 = 0;
                  v71 = 0;
                  __wind
                  {
                    v73 = v70;
                    *((_QWORD *)&v70 + 1) = v70;
                  }
                  __unwind
                  {
                    sub_1435A1930(&v70);
                  }
                  __wind
                  {
                    if ( (unsigned __int8)sub_1435AB1D0(a1, a2, &v70) != 0 )
                    {
                      if ( *(_QWORD *)(a3 + 56) == *(_QWORD *)(a3 + 64) )
                      {
                        sub_14359E600(a3 + 48, *(_QWORD *)(a3 + 56), &v70);
                      }
                      else
                      {
                        sub_1435A0ED0(*(_QWORD *)(a3 + 56), &v70);
                        *(_QWORD *)(a3 + 56) += 24LL;
                      }
                    }
                  }
                  __unwind
                  {
                    sub_1435A19E0(&v70);
                  }
                  sub_1435A3070(&v70);
                }
                else
                {
                  v56 = (char *)sub_146E8C7D0(&unk_149E4B810);
                  v57 = -1;
                  do
                    ++v57;
                  while ( *(_WORD *)&v56[2 * v57] != 0 );
                  v58 = (char *)v74;
                  if ( v76 >= 8 )
                    v58 = (char *)v74[0];
                  v51 = v75;
                  if ( v75 == v57 )
                  {
                    if ( v75 != 0 )
                    {
                      v59 = *(_WORD *)v58;
                      if ( *(_WORD *)v58 >= *(_WORD *)v56 )
                      {
                        v60 = v58 - v56;
                        v61 = v59 <= *(_WORD *)v56;
                        do
                        {
                          if ( !v61 )
                            break;
                          if ( v51 == 1 )
                            goto LABEL_99;
                          --v51;
                          v56 += 2;
                          v62 = *(_WORD *)&v56[v60];
                          v61 = v62 <= *(_WORD *)v56;
                        }
                        while ( v62 >= *(_WORD *)v56 );
                      }
                    }
                    else
                    {
LABEL_99:
                      *(_BYTE *)(a3 + 72) = 1;
                    }
                  }
                }
              }
              else
              {
LABEL_83:
                sub_14359DC90(*(_QWORD *)(a3 + 24), *(_QWORD *)(a3 + 32), a3 + 24, v37);
                *(_QWORD *)(a3 + 32) = *(_QWORD *)(a3 + 24);
                sub_1435A99A0(a1, a2, a3);
              }
            }
            else
            {
LABEL_80:
              LOBYTE(v31) = 1;
              if ( (unsigned __int8)sub_14709DC20(a2, &i, v31, v30) == 0 )
                goto LABEL_101;
              *(_DWORD *)(a3 + 16) = i;
              LOBYTE(v54) = 1;
              if ( (unsigned __int8)sub_14709DC20(a2, &i, v54, v55) == 0 )
                goto LABEL_101;
              *(_DWORD *)(a3 + 20) = i;
            }
          }
          else
          {
LABEL_77:
            LOBYTE(v24) = 1;
            if ( (unsigned __int8)sub_14709DC20(a2, &i, v24, v23) == 0 )
              goto LABEL_101;
            *(_DWORD *)(a3 + 8) = i;
            LOBYTE(v52) = 1;
            if ( (unsigned __int8)sub_14709DC20(a2, &i, v52, v53) == 0 )
              goto LABEL_101;
            *(_DWORD *)(a3 + 12) = i;
            *(_DWORD *)(a3 + 16) = *(_DWORD *)(a3 + 8) - 38;
            *(_DWORD *)(a3 + 20) = 10;
          }
        }
        if ( v75 != 0 )
        {
          v18 = *v16;
          if ( *v16 >= *(_WORD *)v14 )
          {
            v16 = (unsigned __int16 *)((char *)v16 - v14);
            v19 = v18 <= *(_WORD *)v14;
            do
            {
              if ( !v19 )
                break;
              if ( v17 == 1 )
                goto LABEL_75;
              --v17;
              v14 += 2;
              v20 = *(unsigned __int16 *)((char *)v16 + v14);
              v19 = v20 <= *(_WORD *)v14;
            }
            while ( v20 >= *(_WORD *)v14 );
          }
          goto LABEL_25;
        }
LABEL_75:
        LOBYTE(v17) = 1;
        if ( (unsigned __int8)sub_14709DC20(a2, &i, v17, v16) == 0 )
          goto LABEL_101;
      }
    }
    __unwind
    {
      unknown_libname_4(v77);
    }
LABEL_116:
    if ( v79 >= 8 )
    {
      v64 = 2 * v79 + 2;
      v65 = v77[0];
      if ( v64 >= 0x1000 )
      {
        v64 = 2 * v79 + 41;
        v65 = *(_QWORD *)(v77[0] - 8LL);
        if ( (unsigned __int64)(v77[0] - v65 - 8) > 0x1F )
          invalid_parameter_noinfo_noreturn();
      }
      j_j_scalable_free(v65, v64);
    }
    v78 = 0;
    v79 = 7;
    LOWORD(v77[0]) = 0;
  }
  __unwind
  {
    unknown_libname_4(v74);
  }
  if ( v76 >= 8 )
  {
    v66 = 2 * v76 + 2;
    v67 = v74[0];
    if ( v66 >= 0x1000 )
    {
      v66 = 2 * v76 + 41;
      v67 = *(_QWORD *)(v74[0] - 8LL);
      if ( (unsigned __int64)(v74[0] - v67 - 8) > 0x1F )
        invalid_parameter_noinfo_noreturn();
    }
    j_j_scalable_free(v67, v66);
  }
  v75 = 0;
  v76 = 7;
  LOWORD(v74[0]) = 0;
  return v63;
}
