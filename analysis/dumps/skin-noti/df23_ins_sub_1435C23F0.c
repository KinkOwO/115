// ins_sub_1435C23F0

void __fastcall sub_1435C23F0(__int64 a1, __int64 a2, __int64 a3)
{
  __int64 v4; // rsi
  int v5; // r13d
  __int64 v6; // r12
  _QWORD *v7; // rbp
  __int64 v8; // rax
  __int64 v9; // rdx
  int v10; // eax
  __int64 v11; // rcx
  unsigned __int16 v12; // bx
  __int64 v13; // rax
  __int64 v14; // rcx
  __int64 v15; // rax
  __int64 v16; // rcx
  __int64 v17; // rax
  __int64 v18; // rcx
  __int64 v19; // rax
  __int64 v20; // rcx
  __int64 v21; // rax
  __int64 v22; // rcx
  int v23; // r14d
  __int64 v24; // r8
  int v25; // edi
  __int64 v26; // rbx
  __int64 v27; // r9
  __int64 v28; // rax
  __int64 v29; // rcx
  __int64 v30; // rax
  __int64 v31; // rdx
  __int64 v32; // rcx
  __int64 *v33; // r8
  __int64 *v34; // rcx
  __int64 *v35; // rdx
  signed __int8 v36; // r10
  signed __int8 v37; // r9
  bool v38; // cc
  char v39; // al
  bool v40; // cc
  char v41; // al
  int v42; // edi
  __int64 v43; // r8
  __int64 v44; // rbx
  __int64 v45; // rcx
  __int64 v46; // rax
  char *v47; // rbx
  char *v48; // rdi
  __int64 v49; // rcx
  unsigned __int64 v50; // rdx
  __int64 v51; // r8
  __int64 v52; // rcx
  __int64 v53; // rcx
  unsigned __int64 v54; // rdx
  __int64 v55; // r8
  __int64 v56; // rcx
  __int64 v57; // rcx
  unsigned __int64 v58; // rdx
  __int64 v59; // r8
  __int64 v60; // rcx
  __int64 v61; // rcx
  unsigned __int64 v62; // rdx
  __int64 v63; // r8
  __int64 v64; // rcx
  char *v65; // rax
  __int64 v66; // rbx
  _QWORD *v67; // rdi
  __int64 v68; // rbx
  __int64 v69; // rcx
  __int64 v70; // rax
  __int128 v71; // [rsp+28h] [rbp-60h] BYREF
  __int64 v72; // [rsp+38h] [rbp-50h]
  int v73; // [rsp+90h] [rbp+8h]
  __int64 v74; // [rsp+A0h] [rbp+18h]

  if ( *(_BYTE *)(a1 + 1524) != 1 )
  {
    v4 = 0;
    v5 = 0;
    v73 = 0;
    v6 = 0;
    v74 = 0;
    while ( 1 )
    {
      LOBYTE(a3) = 1;
      v7 = (_QWORD *)sub_140283D60(qword_14E683B38, *(unsigned int *)(a1 + 4 * v6 + 1648), a3);
      if ( !v7 )
        break;
      v8 = *(_QWORD *)(32 * v6 + a1 + 1568);
      if ( v8 && *(_DWORD *)(v8 + 8) )
        v9 = *(_QWORD *)(32 * v6 + a1 + 1576);
      else
        v9 = 0;
      v10 = sub_145AD5C20(qword_14E683C80, v9);
      v12 = v10;
      if ( v10 == -1 )
        break;
      v13 = sub_146D74000(v11);
      sub_146D746E0(v13, 160);
      v15 = sub_146D74000(v14);
      sub_146D76180(v15, v12);
      v17 = sub_146D74000(v16);
      sub_146D75CE0(v17, 1);
      v19 = sub_146D74000(v18);
      sub_146D75CC0(v19, *(unsigned __int8 *)(a1 + 4 * v6 + 1672));
      v21 = sub_146D74000(v20);
      sub_146D75CC0(v21, *(unsigned __int8 *)(a1 + 4 * v6 + 1684));
      if ( v5 )
      {
        v22 = (unsigned int)(v5 - 1);
        if ( v5 == 1 )
        {
          v71 = 0;
          v72 = 0;
          v33 = (__int64 *)v7[334];
          v34 = (__int64 *)v33[1];
          v35 = v33;
          v36 = *(_DWORD *)(a1 + 4 * v6 + 1684);
          v37 = *(_DWORD *)(a1 + 4 * v6 + 1672);
          if ( !*((_BYTE *)v34 + 25) )
          {
            do
            {
              v38 = *((_BYTE *)v34 + 32) < (unsigned __int8)v37;
              if ( *((_BYTE *)v34 + 32) == v37
                && (v38 = *((_BYTE *)v34 + 33) < (unsigned __int8)v36, *((_BYTE *)v34 + 33) == v36) )
              {
                v39 = 0;
              }
              else
              {
                v39 = 1;
                if ( v38 )
                  v39 = -1;
              }
              if ( v39 >= 0 )
              {
                v33 = v34;
                v34 = (__int64 *)*v34;
              }
              else
              {
                v34 = (__int64 *)v34[2];
              }
            }
            while ( !*((_BYTE *)v34 + 25) );
            v35 = (__int64 *)v7[334];
          }
          if ( !*((_BYTE *)v33 + 25) )
          {
            v40 = v37 < *((_BYTE *)v33 + 32);
            if ( v37 == *((_BYTE *)v33 + 32) && (v40 = v36 < *((_BYTE *)v33 + 33), v36 == *((_BYTE *)v33 + 33)) )
            {
              v41 = 0;
            }
            else
            {
              v41 = 1;
              if ( v40 )
                v41 = -1;
            }
            if ( v41 >= 0 && v33 != v35 )
              sub_1435BEEB0(&v71, v33 + 5);
          }
          v42 = 0;
          v43 = v71;
          v22 = *((_QWORD *)&v71 + 1) - v71;
          if ( (*((_QWORD *)&v71 + 1) - (_QWORD)v71) / 144LL )
          {
            v44 = 0;
            do
            {
              sub_1435BF210(a1, *(unsigned int *)(v43 + v44));
              v46 = sub_146D74000(v45);
              sub_146D75CE0(v46, *(unsigned int *)(v71 + v44));
              ++v42;
              v44 += 144;
              v43 = v71;
              v22 = *((_QWORD *)&v71 + 1) - v71;
            }
            while ( v42 < (unsigned __int64)((*((_QWORD *)&v71 + 1) - (_QWORD)v71) / 144LL) );
          }
          v47 = (char *)v71;
          if ( (_QWORD)v71 )
          {
            v48 = (char *)*((_QWORD *)&v71 + 1);
            if ( (_QWORD)v71 != *((_QWORD *)&v71 + 1) )
            {
              do
              {
                v49 = *((_QWORD *)v47 + 15);
                if ( v49 )
                {
                  v50 = 4 * ((*((_QWORD *)v47 + 17) - v49) >> 2);
                  if ( v50 >= 0x1000 )
                  {
                    v50 += 39LL;
                    v51 = *(_QWORD *)(v49 - 8);
                    v52 = v49 - v51;
                    if ( (unsigned __int64)(v52 - 8) > 0x1F )
                      sub_148AAF304(v52, v50);
                    v49 = v51;
                  }
                  sub_146E9F3A0(v49, v50);
                  *((_QWORD *)v47 + 15) = 0;
                  *((_QWORD *)v47 + 16) = 0;
                  *((_QWORD *)v47 + 17) = 0;
                }
                v53 = *((_QWORD *)v47 + 11);
                if ( v53 )
                {
                  v54 = 4 * ((*((_QWORD *)v47 + 13) - v53) >> 2);
                  if ( v54 >= 0x1000 )
                  {
                    v54 += 39LL;
                    v55 = *(_QWORD *)(v53 - 8);
                    v56 = v53 - v55;
                    if ( (unsigned __int64)(v56 - 8) > 0x1F )
                      sub_148AAF304(v56, v54);
                    v53 = v55;
                  }
                  sub_146E9F3A0(v53, v54);
                  *((_QWORD *)v47 + 11) = 0;
                  *((_QWORD *)v47 + 12) = 0;
                  *((_QWORD *)v47 + 13) = 0;
                }
                v57 = *((_QWORD *)v47 + 6);
                if ( v57 )
                {
                  v58 = 8 * ((*((_QWORD *)v47 + 8) - v57) >> 3);
                  if ( v58 >= 0x1000 )
                  {
                    v58 += 39LL;
                    v59 = *(_QWORD *)(v57 - 8);
                    v60 = v57 - v59;
                    if ( (unsigned __int64)(v60 - 8) > 0x1F )
                      sub_148AAF304(v60, v58);
                    v57 = v59;
                  }
                  sub_146E9F3A0(v57, v58);
                  *((_QWORD *)v47 + 6) = 0;
                  *((_QWORD *)v47 + 7) = 0;
                  *((_QWORD *)v47 + 8) = 0;
                }
                v61 = *((_QWORD *)v47 + 3);
                if ( v61 )
                {
                  v62 = (*((_QWORD *)v47 + 5) - v61) & 0xFFFFFFFFFFFFFFF8uLL;
                  if ( v62 >= 0x1000 )
                  {
                    v62 += 39LL;
                    v63 = *(_QWORD *)(v61 - 8);
                    v64 = v61 - v63;
                    if ( (unsigned __int64)(v64 - 8) > 0x1F )
                      goto LABEL_78;
                    v61 = v63;
                  }
                  sub_146E9F3A0(v61, v62);
                  *((_QWORD *)v47 + 3) = 0;
                  *((_QWORD *)v47 + 4) = 0;
                  *((_QWORD *)v47 + 5) = 0;
                }
                v47 += 144;
              }
              while ( v47 != v48 );
              v47 = (char *)v71;
            }
            v64 = v72 - (_QWORD)v47;
            v62 = 144 * ((v72 - (__int64)v47) / 144);
            v65 = v47;
            if ( v62 >= 0x1000 )
            {
              v62 += 39LL;
              v47 = (char *)*((_QWORD *)v47 - 1);
              if ( (unsigned __int64)(v65 - v47 - 8) > 0x1F )
LABEL_78:
                sub_148AAF304(v64, v62);
            }
            sub_146E9F3A0(v47, v62);
            v71 = 0;
            v72 = 0;
          }
        }
        else if ( v5 == 2 )
        {
          v23 = 0;
          v24 = v7[329];
          v22 = v7[330] - v24;
          if ( v22 / 80 )
          {
            do
            {
              v25 = 0;
              if ( (*(_QWORD *)(v4 + v24 + 56) - *(_QWORD *)(v4 + v24 + 48)) / 144LL )
              {
                v26 = 0;
                v27 = *(_QWORD *)(v4 + v7[329] + 48);
                do
                {
                  sub_1435BF210(a1, *(unsigned int *)(v26 + v27));
                  ++v25;
                  v26 += 144;
                  v24 = v7[329];
                  v27 = *(_QWORD *)(v4 + v24 + 48);
                }
                while ( v25 < (unsigned __int64)((*(_QWORD *)(v4 + v24 + 56) - v27) / 144) );
              }
              ++v23;
              v4 += 80;
              v22 = v7[330] - v24;
            }
            while ( v23 < (unsigned __int64)(v22 / 80) );
            v6 = v74;
            v5 = v73;
            v4 = 0;
          }
        }
      }
      else
      {
        v66 = 0;
        v67 = (_QWORD *)(a1 + 1936);
        while ( !(unsigned __int8)sub_146F03E70(*v67) )
        {
          ++v66;
          v67 += 17;
          if ( v66 >= 6 )
            goto LABEL_18;
        }
        v68 = (unsigned int)sub_1421B2820(*(_QWORD *)(136 * (v66 + 14) + a1));
        sub_1435BF210(a1, v68);
        v70 = sub_146D74000(v69);
        sub_146D75CE0(v70, (unsigned int)v68);
      }
LABEL_18:
      v28 = sub_146D74000(v22);
      sub_146D75CC0(v28, 0);
      v30 = sub_146D74000(v29);
      sub_146D75CC0(v30, 0);
      sub_146D75AF0(v32, v31);
      v73 = ++v5;
      v74 = ++v6;
      if ( v5 >= 3 )
      {
        *(_BYTE *)(a1 + 1524) = 1;
        return;
      }
    }
  }
}

